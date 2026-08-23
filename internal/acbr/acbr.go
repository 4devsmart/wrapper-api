// Package acbr é o binding para as ACBrLib (bibliotecas fiscais brasileiras,
// ABI C, compiladas em Free Pascal), na variante MultiThread (MT).
//
// Multi-serviço: cada documento (NFSe/CTe/MDFe) é uma .so e uma API separadas
// (NFSE_*/libacbrnfse64, CTE_*/libacbrcte64, MDFE_*/libacbrmdfe64), mas todas
// compartilham o mesmo padrão de sessão MT (Inicializar → config → CarregarINI
// → operar → Finalizar). A fachada expõe um Servico por documento.
//
// Duas implementações selecionadas por build tag:
//
//   - sem tag (padrão):    stubs que reportam a lib indisponível. Permite
//     compilar/rodar a API (healthcheck, contrato, tradução INI) sem os .so.
//   - build tag "acbrlib": binding cgo real. Compilar com
//     CGO_ENABLED=1 go build -tags acbrlib ./...
package acbr

import (
	"errors"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// ErrUnavailable é retornado pelos stubs quando a lib nativa não foi compilada.
var ErrUnavailable = errors.New("ACBrLib indisponível: compile com -tags acbrlib e CGO_ENABLED=1")

// ErrIndeterminado marca a falha em que NÃO se sabe o desfecho da operação: o
// pedido chegou a sair, mas a resposta se perdeu (worker crashou no meio,
// timeout). Numa emissão isso significa que o documento PODE estar na SEFAZ:
// repetir duplicaria. É deliberadamente distinta de ErrUnavailable (onde a
// chamada nunca saiu e repetir é seguro).
var ErrIndeterminado = errors.New("resultado indeterminado: a operação pode ter sido transmitida")

// ErrNaoSuportado marca a operação que a lib não expõe PARA AQUELE DOCUMENTO:
// não é falta da lib, é falta da função. O caso concreto é ValidarRegras na
// NFS-e: o ACBrNFSeX não exporta validação de regras de negócio, enquanto CT-e e
// MDF-e exportam. Distinta de ErrUnavailable de propósito: ali a lib não está
// disponível (503); aqui ela está, e a operação é que não existe (422).
var ErrNaoSuportado = errors.New("operação não suportada por este documento")

// ConfigKV é um par seção/chave/valor de configuração do ACBr (ex.: seção
// "DFe", chave "ArquivoPFX"). Permite ao chamador definir chaves específicas
// (ambiente, provedor, município) sem acoplar o binding a elas.
type ConfigKV struct {
	Section string
	Key     string
	Value   string
}

// TenantConfig descreve a configuração de uma empresa para uma operação.
// O certificado A1 (.pfx) é informado em base64 (PFXBase64) com a senha.
// SSL no Linux é configurado automaticamente (OpenSSL/LibXml2).
type TenantConfig struct {
	CNPJ      string
	PFXBase64 string // conteúdo do .pfx em base64 (vazio = sem certificado)
	SenhaPFX  string
	Config    []ConfigKV // chaves adicionais (Ambiente, CodigoMunicipio, etc.)
}

// Result é o retorno cru de uma operação da ACBrLib: o código de retorno e a
// resposta (JSON ou INI, conforme a config TipoResposta da lib).
type Result struct {
	Codigo   int
	Resposta string
	XML      string // XML do documento (preenchido na emissão bem-sucedida)
	PDF      []byte // PDF (DANFSE/DACTE/DAMDFE) preenchido quando gerado
}

// Servico é a fachada comum de um documento fiscal. As operações recebem o
// TenantConfig para configurar a instância correta (no modo MT, um handle por
// empresa). NFSe e CTe usam exatamente este conjunto; o MDFe o estende.
type Servico interface {
	// Backend descreve a implementação ativa ("stub" ou "cgo").
	Backend() string
	// Version retorna a versão da ACBrLib carregada.
	Version() (string, error)
	// Emitir carrega o INI do documento e o transmite à SEFAZ/provedor.
	Emitir(t TenantConfig, ini string) (Result, error)
	// MontarXML carrega o INI e devolve o XML montado pela lib SEM transmitir
	// (CarregarINI → ObterXml). É a GERAÇÃO: o cliente recebe o
	// documento e a sua chave antes de qualquer byte sair para a SEFAZ, e é isso
	//, não a assinatura, que torna uma transmissão perdida recuperável.
	//
	// NÃO assina: nenhuma lib assina em CarregarINI, e a NFS-e sequer expõe
	// assinatura separada. Por isso gerar dispensa certificado.
	MontarXML(t TenantConfig, ini string) (Result, error)
	// ValidarRegras roda as validações de REGRAS DE NEGÓCIO da lib sobre o INI
	// carregado, sem certificado e sem rede: as rejeições que a SEFAZ daria,
	// antecipadas na geração.
	//
	// NÃO é validação de XSD: o schema exige a tag Signature, então a estrutura
	// só pode ser validada depois de assinar, o que acontece na transmissão, ainda
	// antes de transmitir. Devolve ErrNaoSuportado onde a lib não expõe a função
	// (NFS-e).
	ValidarRegras(t TenantConfig, ini string) (Result, error)
	// Transmitir carrega um XML JÁ MONTADO (o da geração) e o envia ao webservice
	// (CarregarXML → Enviar/Emitir). É a TRANSMISSÃO: o certificado chega aqui, e é
	// a lib que assina no envio.
	//
	// Como qualquer envio, NUNCA deve ser repetido às cegas: em falha
	// indeterminada (ErrIndeterminado) o documento pode ter sido transmitido, e
	// a recuperação correta é Consultar pela chave.
	Transmitir(t TenantConfig, xml string) (Result, error)
	// Consultar consulta um documento pela chave de acesso.
	Consultar(t TenantConfig, chave string) (Result, error)
	// Cancelar envia o evento de cancelamento. Recebe o INI do evento.
	Cancelar(t TenantConfig, ini string) (Result, error)
	// ObterPDF retorna o PDF (base64) da representação gráfica pela chave
	// (DANFSE/DACTE/DAMDFE).
	ObterPDF(t TenantConfig, chave string) (Result, error)
	// RenderizarPDF gera a representação gráfica a partir do XML JÁ AUTORIZADO
	// (CarregarXML → SalvarPDF), fora do fluxo de emissão.
	//
	// Existe por segurança fiscal: o render (FortesReport/GTK2) é o componente
	// que mais crasha, e antes ele rodava DENTRO da sessão de emissão, logo após
	// a transmissão: um SIGSEGV ali matava o processo com o documento já
	// autorizado na SEFAZ e nada persistido. Separado, o pior caso é perder o
	// PDF, que se regenera a partir do XML.
	RenderizarPDF(t TenantConfig, xml string) (Result, error)
	// XmlParaIni carrega um XML e devolve o INI canônico (diagnóstico).
	XmlParaIni(t TenantConfig, xml string) (Result, error)
	// Close libera recursos.
	Close() error
}

// DistDFeModo seleciona o cursor de uma consulta de Distribuição DF-e.
type DistDFeModo string

const (
	DistUltNSU DistDFeModo = "ultnsu" // a partir do último NSU (paginação de lote)
	DistNSU    DistDFeModo = "nsu"    // o DF-e vinculado a um NSU específico
	DistChave  DistDFeModo = "chave"  // o documento pela chave de acesso
)

// DistDFeParams são os parâmetros de uma Distribuição DF-e, em termos primitivos
// (o binding não conhece o domínio). UFAutor (cUF do autor) é exigido por CT-e e
// NFe; o MDF-e o ignora. NSU é string: a lib aceita com ou sem zero-padding.
type DistDFeParams struct {
	UFAutor int
	CNPJCPF string
	Modo    DistDFeModo
	NSU     string // DistUltNSU / DistNSU
	Chave   string // DistChave
}

// NFSeServico estende Servico com a substituição de NFS-e (NFSE_SubstituirNFSe),
// disponível em provedores que expõem o webservice SubstituiNFSe.
type NFSeServico interface {
	Servico
	// SubstituirNFSe substitui uma NFS-e: carrega a DPS substituta (iniNovaDPS,
	// via NFSE_CarregarINI) e envia a substituição identificando a NFS-e antiga
	// (sub). O componente exige a DPS nova carregada antes (NotasFiscais.Count>0).
	SubstituirNFSe(t TenantConfig, iniNovaDPS string, sub SubstituicaoNFSe) (Result, error)
	// ConsultarDFe consulta a Distribuição DF-e do ADN (Padrão Nacional) por NSU.
	// O CNPJ vem da config do tenant (não é parâmetro). A resposta crua é
	// interpretada por internal/distribuicao.ParseNFSe.
	ConsultarDFe(t TenantConfig, nsu int) (Result, error)
	// ConsultarDPSPorChave consulta a NFS-e gerada a partir de uma DPS, usando a
	// chave DA DPS: a que a geração devolve, antes de qualquer transmissão.
	//
	// É a recuperação da NFS-e: sem estado no servidor, um envio cujo desfecho se
	// perdeu só é resolvível perguntando ao ADN se aquela DPS virou nota. É o
	// análogo do Consultar por chave do CT-e, e a razão de a geração devolver o
	// identificador da DPS.
	ConsultarDPSPorChave(t TenantConfig, chaveDPS string) (Result, error)
	// ConsultarPorNumero consulta a NFS-e pelo número (paginação por pagina≥1).
	ConsultarPorNumero(t TenantConfig, numero string, pagina int) (Result, error)
	// ConsultarPorFaixa consulta NFS-e numa faixa de números (paginação).
	ConsultarPorFaixa(t TenantConfig, inicial, final string, pagina int) (Result, error)
	// ConsultarPorRps consulta a NFS-e a partir do RPS que a originou.
	ConsultarPorRps(t TenantConfig, numeroRps, serie, tipo, codVerificacao string) (Result, error)
	// ConsultarSituacao consulta a situação de um lote pelo protocolo/numLote.
	ConsultarSituacao(t TenantConfig, protocolo, numLote string) (Result, error)
	// ConsultarLoteRps consulta o resultado de um lote de RPS pelo protocolo/numLote.
	ConsultarLoteRps(t TenantConfig, protocolo, numLote string) (Result, error)
}

// SubstituicaoNFSe identifica a NFS-e antiga numa substituição (evita parâmetros
// posicionais). Os valores vêm da emissão original (número/série/cód. verificação).
type SubstituicaoNFSe struct {
	NumeroNFSe         string
	SerieNFSe          string
	CodigoCancelamento string // default "1"
	MotivoCancelamento string
	NumeroLote         string
	CodigoVerificacao  string
}

// MDFeServico estende Servico com o encerramento e os demais eventos do MDF-e.
type MDFeServico interface {
	Servico
	// Encerrar envia o evento de encerramento do MDF-e. Recebe o INI do evento.
	Encerrar(t TenantConfig, ini string) (Result, error)
	// EnviarEvento transmite um evento genérico do MDF-e (inclusão de condutor/
	// DF-e, etc.) a partir do INI indexado ([EVENTO001]). Mesmo mecanismo do
	// Encerrar/Cancelar (MDFE_CarregarEventoINI + MDFE_EnviarEvento).
	EnviarEvento(t TenantConfig, ini string) (Result, error)
	// DistribuicaoDFe consulta a Distribuição DF-e do MDF-e (MDFs emitidos contra
	// o CNPJ). MDF-e NÃO usa p.UFAutor. A resposta crua (INI do lote) é
	// interpretada por internal/distribuicao.Parse.
	DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error)
	// StatusServico consulta o status do serviço da SEFAZ (MDFE_StatusServico).
	StatusServico(t TenantConfig) (Result, error)
	// ConsultarRecibo consulta o processamento de um lote pelo recibo (assíncrono).
	ConsultarRecibo(t TenantConfig, recibo string) (Result, error)
	// ConsultaNaoEncerrados lista os MDF-e do CNPJ ainda não encerrados
	// (MDFE_ConsultaMDFeNaoEnc): útil para conformidade antes de emitir novos.
	ConsultaNaoEncerrados(t TenantConfig, cnpj string) (Result, error)
	// SalvarEventoPDF gera o DAMDFE do evento (PDF em Result.PDF) a partir do XML
	// do MDF-e + o XML do evento. Render FortesReport (gtk2+Xvfb), como o DANFSE.
	SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error)
}

// CTeServico estende Servico com a Carta de Correção (CC-e, tpEvento 110110).
// O INI segue o formato INDEXADO de evento ([EVENTO001]+[DETEVENTO001..]); ver
// .claude/skills/acbr-especialista/referencia/cte-eventos.md.
type CTeServico interface {
	Servico
	// CartaCorrecao envia a CC-e. Recebe o INI do evento (mesmo mecanismo do
	// Cancelar: CTE_CarregarEventoINI + CTE_EnviarEvento).
	CartaCorrecao(t TenantConfig, ini string) (Result, error)
	// EnviarEvento transmite um evento genérico do CT-e (EPEC, comprovante de
	// entrega, etc.) a partir do INI indexado. Mesmo mecanismo do CartaCorrecao.
	EnviarEvento(t TenantConfig, ini string) (Result, error)
	// DistribuicaoDFe consulta a Distribuição DF-e do CT-e (CTs emitidos contra o
	// CNPJ). Exige p.UFAutor (cUF do autor). A resposta crua (INI do lote) é
	// interpretada por internal/distribuicao.Parse.
	DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error)
	// StatusServico consulta o status do serviço da SEFAZ (CTE_StatusServico).
	StatusServico(t TenantConfig) (Result, error)
	// ConsultarRecibo consulta o processamento de um lote pelo recibo (assíncrono).
	ConsultarRecibo(t TenantConfig, recibo string) (Result, error)
	// ConsultaCadastro consulta o cadastro de um contribuinte na UF. ehIE=true
	// quando nDocumento é Inscrição Estadual; senão é CNPJ/CPF.
	ConsultaCadastro(t TenantConfig, uf, nDocumento string, ehIE bool) (Result, error)
	// SalvarEventoPDF gera o DACTE do evento (PDF em Result.PDF) a partir do XML
	// do CT-e + o XML do evento. Render FortesReport (gtk2+Xvfb), como o DACTE.
	SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error)
}

// NFeServico é o binding da NF-e RESTRITO a Distribuição DF-e + Manifestação do
// Destinatário (receber NF-e emitidas contra o CNPJ e manifestar-se). Por design
// NÃO embute Servico nem expõe emissão: está fora do escopo do projeto. Reusa
// DistDFeParams (CT-e e NF-e compartilham a assinatura, com AcUFAutor).
type NFeServico interface {
	Backend() string
	Version() (string, error)
	// DistribuicaoDFe consulta a Distribuição DF-e da NF-e (documentos emitidos
	// contra o CNPJ + eventos). Exige p.UFAutor. A resposta crua (INI do lote) é
	// interpretada por internal/distribuicao.Parse.
	DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error)
	// Manifestar envia um evento de Manifestação do Destinatário (tpEvento
	// 210200/210210/210220/210240) a partir do INI indexado do evento
	// (NFE_CarregarEventoINI + NFE_EnviarEvento). xJust é obrigatório no 210240.
	Manifestar(t TenantConfig, ini string) (Result, error)
	Close() error
}

// BoletoServico é o binding do boleto bancário (não-fiscal): geração do
// boleto/PDF a partir de um INI combinado ([Cedente]/[Conta]/[Banco]/[TituloN]).
// Não tem o ciclo SEFAZ: é integração bancária; o banco é só config.
type BoletoServico interface {
	Backend() string
	Version() (string, error)
	// GerarPDF carrega o INI (config + títulos) e devolve o PDF do boleto (base64
	// decodificado em Result.PDF). Geração offline, não fala com o banco.
	GerarPDF(t TenantConfig, ini string) (Result, error)
	// GerarRemessa carrega o INI (config + títulos) e gera o arquivo de remessa
	// CNAB (texto em Result.Resposta). numArquivo = sequencial da remessa.
	GerarRemessa(t TenantConfig, ini string, numArquivo int) (Result, error)
	// LerRetorno processa um arquivo de retorno CNAB (conteúdo cru) usando a
	// config do banco (configINI) e devolve o retorno parseado como INI
	// (títulos + ocorrências) em Result.Resposta.
	LerRetorno(t TenantConfig, configINI, retornoConteudo string) (Result, error)
	// Registrar registra o(s) boleto(s) ONLINE via API do banco,
	// usando op.INI (títulos+config) e op.Operacao.
	Registrar(t TenantConfig, op BoletoOnline) (Result, error)
	// ConsultarTitulos consulta títulos por período na API do banco, usando
	// op.ConfigINI (config do banco) e op.FiltroINI (período/filtros).
	ConsultarTitulos(t TenantConfig, op BoletoOnline) (Result, error)
	Close() error
}

// BoletoOnline reúne os parâmetros da integração ONLINE com a API do banco
// , evitando passar crt/key/operacao posicionais.
type BoletoOnline struct {
	INI       string // Registrar: config + títulos a registrar/baixar
	ConfigINI string // ConsultarTitulos: config do banco (credenciais WS)
	FiltroINI string // ConsultarTitulos: [BoletoConsulta] (período/filtros)
	Operacao  int    // Registrar: 0=incluir, 2=baixar
	CertCRT   []byte // certificado mTLS (.crt): pode ser nil
	CertKEY   []byte // chave privada mTLS (.key): pode ser nil
}

// Servicos agrupa os bindings por documento. Campos podem apontar para stubs
// (lib indisponível) sem afetar os demais.
type Servicos struct {
	NFSe   NFSeServico
	CTe    CTeServico
	MDFe   MDFeServico
	NFe    NFeServico
	Boleto BoletoServico
}

// Close libera os recursos de todos os serviços.
func (s *Servicos) Close() error {
	var firstErr error
	closers := []interface{ Close() error }{s.NFSe, s.CTe, s.MDFe, s.NFe, s.Boleto}
	for _, sv := range closers {
		if sv == nil {
			continue
		}
		if err := sv.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// New cria os bindings adequados à configuração e ao build atual.
//
// Havendo workers configurados (ACBR_WORKERS), devolve clientes RPC: a lib
// nativa roda em OUTRO processo (cmd/fiscal-worker) e este aqui não a carrega:
// um SIGSEGV dentro dela deixa de derrubar a API. Sem workers, cai no binding
// local (stub ou cgo, conforme a build tag), como sempre foi.
func New(cfg config.ACBr) *Servicos {
	if len(cfg.Workers) > 0 {
		return newRemotos(cfg)
	}
	return newServicos(cfg)
}

// NewLocal força o binding NO PROCESSO ATUAL (stub ou cgo, conforme a build
// tag), ignorando ACBR_WORKERS. É o que o fiscal-worker usa: ele é o processo
// que carrega a lib nativa, então delegar de novo seria uma recursão infinita.
func NewLocal(cfg config.ACBr) *Servicos {
	return newServicos(cfg)
}
