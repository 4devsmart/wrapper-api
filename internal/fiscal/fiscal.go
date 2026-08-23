// Package fiscal é o vocabulário comum aos módulos de documento (CT-e, MDF-e,
// NFS-e, boletos): o certificado que chega no payload, o ambiente, a montagem
// do TenantConfig e a tradução dos erros da lib para HTTP.
//
// Existe para que essas quatro decisões sejam tomadas UMA vez. Num serviço em
// que cada requisição carrega um certificado alheio, "cada módulo trata do seu
// jeito" é como se vaza um.
package fiscal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
)

// Certificado é o A1 (.pfx) que o cliente envia na chamada que transmite.
//
// Ele NÃO é persistido em lugar nenhum: vai para o worker por socket unix, é
// usado na sessão nativa e morre com ela. Os métodos String/LogValue abaixo
// existem para que um %v ou um slog distraído não o despeje em log: a única
// forma realista de ele escapar deste processo.
type Certificado struct {
	PFXBase64 string `json:"pfx_b64"`
	Senha     string `json:"senha"`
}

// String redige o conteúdo. Sem isto, um fmt.Sprintf("%v", pedido) num handler
// bastaria para gravar o certificado e a senha no log.
func (c Certificado) String() string { return "Certificado{redigido}" }

// LogValue redige no slog (que não usa String() para structs).
func (c Certificado) LogValue() slog.Value { return slog.StringValue("Certificado{redigido}") }

// Informado indica se veio algum certificado.
func (c Certificado) Informado() bool { return strings.TrimSpace(c.PFXBase64) != "" }

// tamanhoMaxPFX limita o .pfx aceito. Um A1 típico tem poucos KB; o teto barra
// corpo absurdo antes de gastar CPU decodificando.
const tamanhoMaxPFX = 512 << 10

// Validar confere presença e forma. Não abre o PFX: quem valida senha e
// conteúdo é a lib nativa, no worker; aqui é só a triagem barata.
func (c Certificado) Validar() error {
	if !c.Informado() {
		return errors.New("certificado.pfx_b64 é obrigatório para transmitir")
	}
	bruto, err := base64.StdEncoding.DecodeString(strings.TrimSpace(c.PFXBase64))
	if err != nil {
		return errors.New("certificado.pfx_b64 não é base64 válido")
	}
	if len(bruto) == 0 {
		return errors.New("certificado.pfx_b64 está vazio")
	}
	if len(bruto) > tamanhoMaxPFX {
		return fmt.Errorf("certificado.pfx_b64 tem %d bytes; o limite é %d", len(bruto), tamanhoMaxPFX)
	}
	if c.Senha == "" {
		return errors.New("certificado.senha é obrigatória")
	}
	return nil
}

// Ambientes aceitos no campo "ambiente" do payload. São os dois únicos valores:
// qualquer outro é erro do cliente, não um sinônimo de homologação.
const (
	AmbienteHomologacao = "homologacao"
	AmbienteProducao    = "producao"
)

// NormalizarAmbiente devolve o ambiente canônico e se o valor era reconhecido.
// Vazio é homologação: é o default declarado no contrato e o lado seguro.
//
// PONTO ÚNICO, e é o que importa aqui: o mesmo campo vira DOIS códigos
// diferentes (o ordinal da sessão nativa e o tpAmb do XML). Enquanto cada um
// normalizava por conta própria, "Producao" abria a sessão em produção e
// escrevia tpAmb=2 no documento: rejeição 252, ou um documento de homologação
// no webservice de produção. Os dois derivam desta função e não têm como
// divergir de novo.
func NormalizarAmbiente(ambiente string) (string, bool) {
	switch a := strings.ToLower(strings.TrimSpace(ambiente)); a {
	case "":
		return AmbienteHomologacao, true
	case AmbienteHomologacao, AmbienteProducao:
		return a, true
	default:
		return AmbienteHomologacao, false
	}
}

// AmbienteOrdinal traduz o ambiente para o valor que a ACBrLib espera na
// configuração da sessão. Produção = 0, homologação = 1 (é a ordem do enum do
// ACBr, não do tpAmb da SEFAZ: invertê-los emitiria em produção achando que era
// homologação).
func AmbienteOrdinal(ambiente string) string {
	if a, _ := NormalizarAmbiente(ambiente); a == AmbienteProducao {
		return "0"
	}
	return "1"
}

// TpAmb traduz o ambiente para o tpAmb que vai NO XML, na convenção da SEFAZ:
// 1 = produção, 2 = homologação. É o oposto do AmbienteOrdinal, e por isso os
// dois moram lado a lado: quem mexer num vê o outro.
func TpAmb(ambiente string) string {
	if a, _ := NormalizarAmbiente(ambiente); a == AmbienteProducao {
		return "1"
	}
	return "2"
}

// AmbienteValido normaliza o ambiente do pedido, respondendo 400 quando o valor
// não é reconhecido. Devolve o canônico, que é o que deve seguir para a sessão
// E para o INI.
//
// Recusar em vez de assumir homologação é deliberado: um "prod" digitado errado
// que virasse homologação em silêncio produziria um documento sem valor fiscal
// com resposta 200.
func AmbienteValido(w http.ResponseWriter, ambiente string) (string, bool) {
	a, ok := NormalizarAmbiente(ambiente)
	if !ok {
		httpx.ErroJSON(w, http.StatusBadRequest, "ambiente_invalido",
			`ambiente deve ser "`+AmbienteHomologacao+`" ou "`+AmbienteProducao+`"; omitir assume homologação`)
		return "", false
	}
	return a, true
}

// Tenant monta a configuração da sessão nativa. secao é a seção de config do
// documento na ACBrLib ("CTe", "MDFe", "NFSe").
//
// cert zerado é legítimo e é o caso da geração: montar e validar não assinam
// nem falam com a SEFAZ, então não precisam de certificado.
func Tenant(cnpj, secao, ambiente string, cert Certificado) acbr.TenantConfig {
	return acbr.TenantConfig{
		CNPJ:      cnpj,
		PFXBase64: strings.TrimSpace(cert.PFXBase64),
		SenhaPFX:  cert.Senha,
		Config:    []acbr.ConfigKV{{Section: secao, Key: "Ambiente", Value: AmbienteOrdinal(ambiente)}},
	}
}

// CNPJDaChave extrai o CNPJ do emitente da chave de acesso (posições 7 a 20 dos
// 44 dígitos). Serve para rotular a sessão nativa quando só temos o XML.
func CNPJDaChave(chave string) string {
	chave = SoDigitos(chave)
	if len(chave) != 44 {
		return ""
	}
	return chave[6:20]
}

// SoDigitos remove tudo que não é dígito.
func SoDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ResponderErroDaLib traduz uma falha do binding em resposta HTTP. A distinção
// entre os casos não é cosmética. Ela diz ao cliente se pode repetir:
//
//   - indisponível: a chamada NÃO saiu; repetir é seguro (503);
//   - não suportado: a lib está viva, a operação não existe para este documento (422);
//   - indeterminado: o pedido PODE ter chegado à SEFAZ; repetir pode duplicar (502);
//   - o resto: a lib falhou de forma conhecida (502).
func ResponderErroDaLib(w http.ResponseWriter, doc string, res acbr.Result, err error) {
	detalhes := detalhesDaLib(res)
	switch {
	case errors.Is(err, acbr.ErrUnavailable):
		httpx.ErroDetalhado(w, http.StatusServiceUnavailable, "lib_indisponivel",
			"o motor fiscal de "+doc+" não está disponível; a chamada não foi enviada", detalhes)
	case errors.Is(err, acbr.ErrNaoSuportado):
		httpx.ErroDetalhado(w, http.StatusUnprocessableEntity, "operacao_nao_suportada",
			"esta operação não existe para "+doc, detalhes)
	case errors.Is(err, acbr.ErrIndeterminado):
		httpx.ErroDetalhado(w, http.StatusBadGateway, "desfecho_indeterminado",
			"a operação pode ter sido transmitida e a resposta se perdeu: NÃO repita: consulte pela chave para saber o desfecho",
			detalhes)
	default:
		httpx.ErroDetalhado(w, http.StatusBadGateway, "falha_na_lib",
			"o motor fiscal de "+doc+" falhou: "+err.Error(), detalhes)
	}
}

func detalhesDaLib(res acbr.Result) any {
	if res.Codigo == 0 && strings.TrimSpace(res.Resposta) == "" {
		return nil
	}
	return map[string]any{"codigo": res.Codigo, "resposta": res.Resposta}
}

// --- gerar o XML -----------------------------------------------

// Validacao é o resultado das regras de negócio da lib.
//
// Suportada existe porque nem todo documento a expõe: a NFS-e não tem. Dizer
// "ok: true" quando validação nenhuma rodou seria mentir sobre a garantia que o
// cliente tem em mãos.
type Validacao struct {
	OK        bool     `json:"ok"`
	Suportada bool     `json:"suportada"`
	Mensagens []string `json:"mensagens,omitempty"`
}

// Montar gera o documento, e é comum a todos: monta o XML a partir do INI e roda
// as regras de negócio. Não assina e não fala com a SEFAZ, por isso o Tenant
// que chega aqui não precisa (nem deve) trazer certificado.
//
// São duas sessões nativas. É deliberado: sem certificado, abrir sessão é
// barato: o custo real, carregar o PFX, só existe na transmissão. Juntar as duas num
// método do binding é otimização a fazer depois de medir.
func Montar(svc acbr.Servico, t acbr.TenantConfig, ini string) (xml string, val Validacao, res acbr.Result, err error) {
	res, err = svc.MontarXML(t, ini)
	if err != nil {
		return "", Validacao{}, res, err
	}
	val, resVal, err := validarRegras(svc, t, ini)
	if err != nil {
		// A validação é a segunda chamada, e falhar nela não é detalhe: o XML
		// existe, mas ninguém conferiu as regras. Devolver o documento com
		// "ok" aqui seria entregar um carimbo que não foi dado.
		return "", Validacao{}, resVal, err
	}
	return res.XML, val, res, nil
}

// validarRegras roda a validação de regras de negócio da lib.
//
// ARMADILHA DA ABI, e ela vale para todo documento: a lib devolve SEMPRE código
// 0 aqui, mesmo com rejeições: ACBrLibCTeBase.pas:554 faz
// SetRetorno(ErrOK, Erros) incondicionalmente, e o MDF-e faz igual. Quem diz que
// passou é a RESPOSTA VIR VAZIA. Conferir o código daria "válido" para qualquer
// documento.
func validarRegras(svc acbr.Servico, t acbr.TenantConfig, ini string) (Validacao, acbr.Result, error) {
	res, err := svc.ValidarRegras(t, ini)
	switch {
	case errors.Is(err, acbr.ErrNaoSuportado):
		// A lib não expõe validação para este documento (NFS-e). Honesto é dizer
		// que não rodou, não fingir que passou.
		//
		// A sentinela é o ÚNICO erro tolerado aqui, e é por isso que ela
		// atravessa o RPC num campo próprio (rpc.go, NaoSuportado). Tratar
		// qualquer erro como "não suportada" transformava worker reiniciado,
		// timeout e vaga esgotada em "validação ok, só não rodou".
		return Validacao{OK: true, Suportada: false}, acbr.Result{}, nil
	case err != nil:
		return Validacao{}, res, err
	}
	msgs := Linhas(res.Resposta)
	return Validacao{OK: len(msgs) == 0, Suportada: true, Mensagens: msgs}, res, nil
}

// --- entrada e saída --------------------------------------------------------

// XMLdeBase64 decodifica o campo xml_b64 de um pedido. Devolve false (e já
// respondeu) quando o cliente errou.
func XMLdeBase64(w http.ResponseWriter, campo, b64 string) (string, bool) {
	bruto, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		httpx.ErroJSON(w, http.StatusBadRequest, "xml_invalido", campo+" não é base64 válido")
		return "", false
	}
	if len(strings.TrimSpace(string(bruto))) == 0 {
		httpx.ErroJSON(w, http.StatusBadRequest, "xml_invalido", campo+" está vazio")
		return "", false
	}
	return string(bruto), true
}

// Base64 codifica um texto para devolver ao cliente.
func Base64(s string) string {
	if s == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// DecodarAninhado decodifica um bloco aninhado do pedido (o "evento"), recusando
// campo desconhecido como no nível de cima. Devolve a mensagem de erro, ou "".
func DecodarAninhado(rotulo string, bruto json.RawMessage, dst any) string {
	if len(bruto) == 0 {
		bruto = json.RawMessage("{}")
	}
	dec := json.NewDecoder(strings.NewReader(string(bruto)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if campo, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return rotulo + ": campo desconhecido " + campo
		}
		return rotulo + ": " + err.Error()
	}
	return ""
}

// --- status HTTP ------------------------------------------------------------

// StatusDoDesfecho mapeia o desfecho de uma transmissão para HTTP. Rejeitado é
// 422 (o documento do cliente não passou), não 200, mas o corpo vai completo,
// com cStat e motivo, porque é isso que ele precisa para corrigir.
func StatusDoDesfecho(status string) int {
	switch status {
	case "autorizado":
		return http.StatusOK
	case "rejeitado":
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadGateway
	}
}

// StatusDoEvento é o equivalente para eventos.
func StatusDoEvento(status string) int {
	switch status {
	case "concluido":
		return http.StatusOK
	case "rejeitado":
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadGateway
	}
}

// --- utilidades -------------------------------------------------------------

// AgoraLocal formata data/hora no formato que o StringToDateTime do ACBr aceita
// (sem fuso; a lib aplica o do estado ao gerar o XML).
func AgoraLocal() string { return time.Now().Format("2006-01-02 15:04:05") }

// Linhas quebra um texto em linhas não vazias (saída de validação da lib).
func Linhas(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Primeiro devolve o primeiro valor não vazio.
func Primeiro(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ChaveValida confere os 44 dígitos da chave de acesso e devolve a normalizada.
func ChaveValida(w http.ResponseWriter, chave string) (string, bool) {
	c := SoDigitos(chave)
	if len(c) != 44 {
		httpx.ErroJSON(w, http.StatusBadRequest, "chave_invalida", "chave deve ter 44 dígitos")
		return "", false
	}
	return c, true
}
