// Package nfse modela a NFS-e (Padrão Nacional) com nomes alinhados ao
// contrato da Nuvem Fiscal (infDPS: prest/toma/serv/valores) e traduz esse
// JSON para o arquivo INI consumido pela ACBrLibNFSe.
//
// A meta do contrato é cobrir o INI INTEIRO que a biblioteca aceita, e não um
// subconjunto dele: campo que a lib lê e o contrato não manda é dado que o
// cliente não consegue declarar, e campo que mandamos e ela não lê é chave
// morta. Quem mede as duas direções é o lockstep (lockstep_test.go), e o que
// ainda falta está em testdata/nao_enviadas.tsv.
package nfse

// DPSPedido é o corpo de emissão (estilo Nuvem Fiscal: POST /v1/nfse/xml).
//
// NÃO existe campo para escolher o provedor. Quem o decide é o município
// (infDPS.cLocEmi, ou o cMun do prestador): a ACBrLib resolve o provedor lendo
// a própria tabela a partir do CodigoMunicipio, e não aceita override. Testado
// contra a lib: NFSE_ConfigGravarValor com a chave "Provedor" falha em todas as
// formas (PadraoNacional, proPadraoNacional). Para saber quem atende um
// município antes de montar, use GET /v1/nfse/municipios/{codigo}.
type DPSPedido struct {
	Ambiente   string `json:"ambiente" enum:"Ambiente"`
	Referencia string `json:"referencia,omitempty"` // id externo do cliente
	InfDPS     InfDPS `json:"infDPS"`
}

// InfDPS são as informações da DPS (Declaração de Prestação de Serviços).
type InfDPS struct {
	Serie    string         `json:"serie"`
	NDPS     string         `json:"nDPS"`                                 // número do RPS/DPS
	DCompet  string         `json:"dCompet" fmt:"data"`                   // competência (YYYY-MM-DD)
	DhEmi    string         `json:"dhEmi,omitempty" fmt:"data"`           // emissão; vazio = hoje
	TpEmit   int            `json:"tpEmit,omitempty" enum:"TipoEmitente"` // 1 = prestador
	VerAplic string         `json:"verAplic,omitempty"`
	CLocEmi  string         `json:"cLocEmi,omitempty"` // município emissor (IBGE); vazio = cMun do prestador
	Prest    Prestador      `json:"prest"`
	Toma     *Tomador       `json:"toma,omitempty"`
	Interm   *Intermediario `json:"interm,omitempty"` // intermediário (seção [Intermediario])
	Serv     Servico        `json:"serv"`
	Valores  Valores        `json:"valores"`
	IBSCBS   *IBSCBSDPS     `json:"ibscbs,omitempty"` // Reforma Tributária (grupo IBSCBSDPS)
	// RpsSubst é o RPS que este documento substitui (seção [RpsSubstituido]).
	RpsSubst *RpsSubstituido `json:"rpsSubstituido,omitempty"`
	// Campos que a seção [IdentificacaoRps] do INI aceita além dos acima. São
	// layout municipal: cada provedor lê os seus. Ver identificacaoRps.
	DataEmissaoRPS            string  `json:"dataEmissaoRPS,omitempty" fmt:"data"`          // emissão do RPS, quando difere da competência
	DataFatoGerador           string  `json:"dataFatoGerador,omitempty" fmt:"data"`         // data do fato gerador
	DataPagamento             string  `json:"dataPagamento,omitempty" fmt:"data"`           // data do pagamento
	Vencimento                string  `json:"vencimento,omitempty" fmt:"data"`              // vencimento do documento
	DhRecebimento             string  `json:"dhRecebimento,omitempty" fmt:"data"`           // data e hora do recebimento
	TipoRecolhimento          string  `json:"tipoRecolhimento,omitempty"`                   // tipo de recolhimento do ISS
	TipoTributacaoRPS         string  `json:"tipoTributacaoRPS,omitempty"`                  // tipo de tributação do RPS
	SituacaoTrib              string  `json:"situacaoTrib,omitempty"`                       // situação tributária do RPS
	Situacao                  int     `json:"situacao,omitempty"`                           // situação do RPS no layout do provedor
	TipoNota                  int     `json:"tipoNota,omitempty"`                           // tipo da nota no layout do provedor
	NumeroParcelas            int     `json:"numeroParcelas,omitempty"`                     // quantidade de parcelas
	FormaPagamento            string  `json:"formaPagamento,omitempty"`                     // forma de pagamento
	EspecieDocumento          string  `json:"especieDocumento,omitempty"`                   // espécie do documento
	SerieTalonario            string  `json:"serieTalonario,omitempty"`                     // série do talonário
	SeriePrestacao            string  `json:"seriePrestacao,omitempty"`                     // série da prestação
	SiglaUF                   string  `json:"siglaUF,omitempty"`                            // UF do RPS no layout do provedor
	OutrasInformacoes         string  `json:"outrasInformacoes,omitempty"`                  // outras informações do RPS
	InformacoesComplementares string  `json:"informacoesComplementares,omitempty"`          // informações complementares do RPS
	IdentificacaoRemessa      string  `json:"identificacaoRemessa,omitempty"`               // identificação da remessa
	EqptoRecibo               string  `json:"eqptoRecibo,omitempty"`                        // equipamento emissor do recibo
	RegRec                    string  `json:"regRec,omitempty"`                             // regime de recolhimento
	FrmRec                    string  `json:"frmRec,omitempty"`                             // forma de recolhimento
	Producao                  int     `json:"producao,omitempty"`                           // 1=Sim, 2=Não, documento de produção
	DeducaoMateriais          int     `json:"deducaoMateriais,omitempty"`                   // 1=Sim, 2=Não, dedução de materiais
	CMotivoEmisTI             string  `json:"cMotivoEmisTI,omitempty"`                      // motivo da emissão em contingência
	IDSisLegado               int     `json:"id_sis_legado,omitempty"`                      // identificador no sistema legado
	PercCargaTribMunicipal    float64 `json:"percentualCargaTributariaMunicipal,omitempty"` // carga tributária municipal (%)
	ValorCargaTribMunicipal   float64 `json:"valorCargaTributariaMunicipal,omitempty"`      // carga tributária municipal
	PercCargaTribEstadual     float64 `json:"percentualCargaTributariaEstadual,omitempty"`  // carga tributária estadual (%)
	ValorCargaTribEstadual    float64 `json:"valorCargaTributariaEstadual,omitempty"`       // carga tributária estadual
	// Grupos do documento que a biblioteca lê em seções próprias.
	CondicaoPagamento *CondicaoPagamento `json:"condicaoPagamento,omitempty"` // parcelamento
	Transportadora    *Transportadora    `json:"transportadora,omitempty"`    // transportadora
	Emails            []string           `json:"emails,omitempty"`            // e-mails em cópia (emailCC)
	Quartos           []Quarto           `json:"quartos,omitempty"`           // diárias de hotelaria
	Despesas          []Despesa          `json:"despesas,omitempty"`          // despesas reembolsáveis
	Genericos         []Generico         `json:"genericos,omitempty"`         // campos livres do provedor
}

// Pessoa representa prestador ou tomador.
type Pessoa struct {
	CNPJ  string `json:"CNPJ,omitempty"`
	CPF   string `json:"CPF,omitempty"`
	IM    string `json:"IM,omitempty"`    // inscrição municipal
	XNome string `json:"xNome,omitempty"` // razão social / nome
	CMun  string `json:"cMun,omitempty"`  // código do município (IBGE)
	UF    string `json:"UF,omitempty"`
	CEP   string `json:"CEP,omitempty"`
	Email string `json:"email,omitempty"`
	// Endereço/contato (Padrão Nacional: seção [Prestador]/[Tomador]).
	Logradouro  string `json:"logradouro,omitempty"`
	Numero      string `json:"numero,omitempty"`
	Complemento string `json:"complemento,omitempty"`
	Bairro      string `json:"bairro,omitempty"`
	Telefone    string `json:"telefone,omitempty"`
	// CPais/XPais só precisam ser informados para endereço NO EXTERIOR, com o
	// código da tabela BACEN, não o ISO (o Brasil é 1058, não 76). Com cMun de
	// município brasileiro o país é sempre 1058.
	CPais int    `json:"cPais,omitempty"` // código do país (1058 = Brasil)
	XPais string `json:"xPais,omitempty"` // nome do país
	// Identificação no exterior: as três seções do INI leem estas três.
	NIF        string `json:"NIF,omitempty"`        // identificação fiscal no exterior
	CNaoNIF    string `json:"cNaoNIF,omitempty"`    // motivo de não informar o NIF
	CAEPF      string `json:"CAEPF,omitempty"`      // cadastro de atividade econômica de pessoa física
	XMunicipio string `json:"xMunicipio,omitempty"` // nome do município do endereço
}

// Prestador é quem emite, com o que a seção [Prestador] do INI aceita além dos
// campos comuns. Cada papel tem a sua struct porque cada seção lê um conjunto
// diferente: campo fora da seção certa vira chave morta.
type Prestador struct {
	Pessoa
	// RegTrib (regime tributário) é obrigatório no Padrão Nacional, sem ele o
	// ACBr falha ao montar o XML.
	RegTrib             *RegTrib `json:"regTrib,omitempty"`
	TipoPessoa          string   `json:"tipoPessoa,omitempty"`                     // tipo de pessoa no layout do provedor
	InscricaoEstadual   string   `json:"inscricaoEstadual,omitempty"`              // inscrição estadual
	NomeFantasia        string   `json:"nomeFantasia,omitempty"`                   // nome fantasia
	TipoLogradouro      string   `json:"tipoLogradouro,omitempty"`                 // tipo do logradouro (Rua, Avenida)
	DDD                 string   `json:"DDD,omitempty"`                            // DDD do telefone
	XSite               string   `json:"xSite,omitempty"`                          // site do prestador
	CRC                 string   `json:"crc,omitempty"`                            // registro no conselho de contabilidade
	CRCEstado           string   `json:"crc_estado,omitempty"`                     // UF do registro no conselho
	Anexo               string   `json:"anexo,omitempty"`                          // anexo do Simples Nacional
	ValorReceitaBruta   float64  `json:"valorReceitaBruta,omitempty"`              // receita bruta acumulada
	DataInicioAtividade string   `json:"dataInicioAtividade,omitempty" fmt:"data"` // início da atividade (YYYY-MM-DD)
	OptanteMEISimei     int      `json:"optanteMEISimei,omitempty"`                // 1=Sim, 2=Não
}

// Tomador é quem contrata, com o que a seção [Tomador] do INI aceita além dos
// campos comuns.
type Tomador struct {
	Pessoa
	TipoPessoa        string `json:"tipoPessoa,omitempty"`                  // tipo de pessoa no layout do provedor
	InscricaoEstadual string `json:"inscricaoEstadual,omitempty"`           // inscrição estadual
	NomeFantasia      string `json:"nomeFantasia,omitempty"`                // nome fantasia
	TipoLogradouro    string `json:"tipoLogradouro,omitempty"`              // tipo do logradouro (Rua, Avenida)
	TipoBairro        string `json:"tipoBairro,omitempty"`                  // tipo do bairro
	PontoReferencia   string `json:"pontoReferencia,omitempty"`             // ponto de referência do endereço
	DDD               string `json:"DDD,omitempty"`                         // DDD do telefone
	TipoTelefone      string `json:"tipoTelefone,omitempty"`                // tipo do telefone
	DocEstrangeiro    string `json:"docEstrangeiro,omitempty"`              // documento do tomador estrangeiro
	EnderecoInformado string `json:"enderecoInformado,omitempty"`           // 1=Sim, 2=Não, informado pelo tomador
	AtualizaTomador   int    `json:"atualizaTomador,omitempty"`             // 1=Sim, 2=Não, atualiza o cadastro
	TomadorExterior   int    `json:"tomadorExterior,omitempty"`             // 1=Sim, 2=Não
	TomadorSubstituto int    `json:"tomadorSubstitutoTributario,omitempty"` // 1=Sim, 2=Não
}

// Intermediario é o intermediário do serviço. A seção [Intermediario] lê os
// campos comuns e mais um.
type Intermediario struct {
	Pessoa
	IssRetido int `json:"issRetido,omitempty"` // 1=Sim, 2=Não
}

// RegTrib é o regime tributário do prestador.
type RegTrib struct {
	OpSimpNac   int `json:"opSimpNac,omitempty" enum:"OpSimplesNacional"`         // 1=não optante, 2=MEI, 3=ME/EPP
	RegApTribSN int `json:"regApTribSN,omitempty" enum:"RegimeApuracaoSN"`        // regime de apuração do Simples Nacional
	RegEspTrib  int `json:"regEspTrib,omitempty" enum:"RegimeEspecialTributacao"` // regime especial de tributação
	// Campos exclusivos ABRASF (municípios fora do Padrão Nacional): opcionais,
	// ignorados na emissão Padrão Nacional.
	IncentCultural int    `json:"incentivadorCultural,omitempty"`                  // 1=Sim, 2=Não
	DataOpSimpNac  string `json:"dataOptanteSimplesNacional,omitempty" fmt:"data"` // YYYY-MM-DD
}

// Servico descreve o serviço prestado.
type Servico struct {
	CMunPrestacao string `json:"cMunPrestacao,omitempty"` // município da prestação (IBGE)
	CServ         string `json:"cServ,omitempty"`         // item da lista (→ cTribNac no Padrão Nacional)
	CTribMun      string `json:"cTribMun,omitempty"`      // código de tributação municipal
	CNBS          string `json:"cNBS,omitempty"`          // código NBS
	CodigoCnae    string `json:"codigoCnae,omitempty"`    // CNAE
	XDescServ     string `json:"xDescServ"`               // discriminação
	// ExigibilidadeISS é campo do padrão ABRASF: o Padrão Nacional usa
	// tribMun.tribISSQN. Os provedores ABRASF esperam a tabela do layout 2.04.
	ExigibISS      int        `json:"exigibilidadeISS,omitempty" enum:"ExigibilidadeISS"`
	MunIncidencia  string     `json:"municipioIncidencia,omitempty"` // município de incidência do ISS (IBGE)
	XMunIncidencia string     `json:"xMunicipioIncidencia,omitempty"`
	NumeroProcesso string     `json:"numeroProcesso,omitempty"` // processo de suspensão/exigibilidade
	CodigoPais     string     `json:"codigoPais,omitempty"`     // país da prestação (exterior)
	XPais          string     `json:"xPais,omitempty"`
	ComExt         *ComExt    `json:"comExt,omitempty"`    // comércio exterior
	InfoCompl      *InfoCompl `json:"infoCompl,omitempty"` // informações complementares
	// Campos exclusivos ABRASF (municípios fora do Padrão Nacional): opcionais,
	// ignorados na emissão Padrão Nacional. Onde houver equivalente PN (cServ,
	// cTribMun, pAliq em valores), o builder ABRASF cai nele como fallback.
	ItemListaServico string `json:"itemListaServico,omitempty"` // LC116 (ex.: "01.07"); fallback: cServ
	RespRetencao     int    `json:"responsavelRetencao,omitempty"`
	NaturezaOperacao int    `json:"naturezaOperacao,omitempty"` // ABRASF v1
	// Campos que a seção [Servico] do INI aceita e que layouts municipais leem.
	// Os nomes seguem a chave do INI, que é o contrato real da biblioteca.
	Descricao          string  `json:"descricao,omitempty"`                  // descrição do serviço; vazio cai na discriminação
	CodigoNCM          string  `json:"codigoNCM,omitempty"`                  // ISSSaoPaulo
	CFPS               string  `json:"CFPS,omitempty"`                       // SoftPlan
	CodigoInterContr   string  `json:"codigoInterContr,omitempty"`           // código interno do contribuinte
	CClassTrib         string  `json:"cClassTrib,omitempty"`                 // ISSSalvador e Reforma Tributária
	INDOP              string  `json:"INDOP,omitempty"`                      // indicador de operação
	IdentifNaoExigib   string  `json:"identifNaoExigibilidade,omitempty"`    // identificador da não exigibilidade do ISS
	InfAdicional       string  `json:"infAdicional,omitempty"`               // MegaSoft
	TipoLancamento     string  `json:"tipoLancamento,omitempty"`             // P = próprio (default da lib)
	Operacao           string  `json:"operacao,omitempty"`                   // ISSDSF
	Tributacao         string  `json:"tributacao,omitempty"`                 // ISSDSF
	LocalPrestacao     string  `json:"localPrestacao,omitempty"`             // ISSBarueri
	PrestadoViasPublic *bool   `json:"prestadoEmViasPublicas,omitempty"`     // ISSBarueri; a lib assume true
	MunPrestacaoServ   string  `json:"municipioPrestacaoServico,omitempty"`  // nome do município da prestação
	UFPrestacao        string  `json:"UFPrestacao,omitempty"`                // UF do local da prestação
	PercCargaTrib      float64 `json:"percentualCargaTributaria,omitempty"`  // ISSSaoPaulo
	ValorCargaTrib     float64 `json:"valorCargaTributaria,omitempty"`       // ISSSaoPaulo
	FonteCargaTrib     string  `json:"fonteCargaTributaria,omitempty"`       // ISSSaoPaulo
	ValorTotalRecebido float64 `json:"valorTotalRecebido,omitempty"`         // ISSSaoPaulo
	XFormaPagamento    string  `json:"xFormaPagamento,omitempty"`            // SigISSWeb
	XItemListaServico  string  `json:"xItemListaServico,omitempty"`          // descrição do item da lista de serviços
	XCodigoTribMun     string  `json:"xCodigoTributacaoMunicipio,omitempty"` // descrição do código de tributação municipal
	XNBS               string  `json:"xNBS,omitempty"`                       // descrição do código NBS
	XMunicipio         string  `json:"xMunicipio,omitempty"`                 // nome do município do endereço da prestação
	// EndPrestacao é o endereço do local da prestação, que a biblioteca lê
	// achatado dentro de [Servico] (usado pelo Giap, entre outros).
	EndPrestacao *EnderecoPrestacao `json:"enderecoPrestacao,omitempty"`
	// Grupos do serviço que a biblioteca lê em seções próprias.
	Deducoes   []Deducao   `json:"deducoes,omitempty"`   // deduções por documento referenciado
	Impostos   []Imposto   `json:"impostos,omitempty"`   // quebra de imposto do serviço
	Locacao    *Locacao    `json:"locacao,omitempty"`    // locação e sublocação de postes e dutos
	Rodoviaria *Rodoviaria `json:"rodoviaria,omitempty"` // exploração de rodovia
	// DocsDeducao são os documentos que embasam dedução ou redução da base.
	DocsDeducao []DocDeducaoReducao `json:"documentosDeducao,omitempty"`
	// EnderecoServico é o endereço de EXECUÇÃO do serviço, que a biblioteca lê
	// por item do RPS. Não confundir com enderecoPrestacao, que vai achatado em
	// [Servico].
	EnderecoServico *EnderecoServico `json:"enderecoServico,omitempty"`
	// ConstrucaoCivil é a obra a que o serviço se refere.
	ConstrucaoCivil *ConstrucaoCivil `json:"construcaoCivil,omitempty"`
	// Evento é a atividade de evento a que o serviço se refere.
	Evento *EventoServico `json:"evento,omitempty"`
	// Lista de serviços do RPS. Informada, é ela que vai ao documento; sem ela,
	// o item é derivado deste serviço (ver itemServico).
	Itens []ItemServico `json:"itens,omitempty"`
}

// EnderecoPrestacao é o endereço do local da prestação do serviço, lido das
// chaves de endereço da seção [Servico].
type EnderecoPrestacao struct {
	Logradouro     string `json:"logradouro,omitempty"`
	TipoLogradouro string `json:"tipoLogradouro,omitempty"` // tipo do logradouro (Rua, Avenida)
	Numero         string `json:"numero,omitempty"`
	Complemento    string `json:"complemento,omitempty"`
	Bairro         string `json:"bairro,omitempty"`
	CEP            string `json:"CEP,omitempty"`
	UF             string `json:"UF,omitempty"`
}

// ComExt é o grupo de comércio exterior (seção [ComercioExterior]).
type ComExt struct {
	MdPrestacao string  `json:"mdPrestacao,omitempty"` // modo de prestação
	VincPrest   string  `json:"vincPrest,omitempty"`   // vínculo da prestação
	TpMoeda     int     `json:"tpMoeda,omitempty"`     // código da moeda
	VServMoeda  float64 `json:"vServMoeda,omitempty"`  // valor do serviço em moeda estrangeira
	MecAFComexP string  `json:"mecAFComexP,omitempty"`
	MecAFComexT string  `json:"mecAFComexT,omitempty"`
	MovTempBens string  `json:"movTempBens,omitempty"`
	NDI         string  `json:"nDI,omitempty"` // nº da Declaração de Importação
	NRE         string  `json:"nRE,omitempty"` // nº do Registro de Exportação
	Mdic        int     `json:"mdic,omitempty"`
}

// InfoCompl são informações complementares (seção [InformacoesComplementares]).
type InfoCompl struct {
	IdDocTec string   `json:"idDocTec,omitempty"`
	DocRef   string   `json:"docRef,omitempty"`
	XPed     string   `json:"xPed,omitempty"`
	XInfComp string   `json:"xInfComp,omitempty"`
	GItemPed []string `json:"gItemPed,omitempty"` // itens do pedido ([gItemPedNN] xItemPed)
}

// Valores são os montantes e a tributação do serviço (ISSQN + federal + totais).
type Valores struct {
	VServ       float64 `json:"vServ"`                 // valor dos serviços
	VReceb      float64 `json:"vReceb,omitempty"`      // valor recebido
	VDescIncond float64 `json:"vDescIncond,omitempty"` // desconto incondicionado
	VDescCond   float64 `json:"vDescCond,omitempty"`   // desconto condicionado
	VDeducoes   float64 `json:"vDeducoes,omitempty"`   // valor das deduções
	PDeducoes   float64 `json:"pDeducoes,omitempty"`   // alíquota de deduções (%)
	// ISSQN (atalhos comuns; o detalhe vai em tribMun).
	TribISSQN int     `json:"tribISSQN,omitempty" enum:"TributacaoISSQN"` // 1=tributável,2=não incidência,...
	PAliq     float64 `json:"pAliq,omitempty"`                            // alíquota ISS (%)
	IssRetido int     `json:"iss_retido,omitempty" enum:"ISSRetido"`      // 1=Sim, 2=Não (ABRASF); default 2
	// Tributação detalhada (Padrão Nacional).
	TribMun *TribMun `json:"tribMun,omitempty"`
	TribFed *TribFed `json:"tribFed,omitempty"`
	TotTrib *TotTrib `json:"totTrib,omitempty"`
	// Campos que a seção [Valores] do INI aceita e que layouts municipais leem.
	// Os nomes seguem a chave do INI, que é o contrato real da biblioteca.
	BaseCalculo          float64 `json:"baseCalculo,omitempty"`              // base do ISS (ABRASF v1 e layouts próprios)
	ValorIss             float64 `json:"valorIss,omitempty"`                 // ISS calculado
	ValorIssRetido       float64 `json:"valorIssRetido,omitempty"`           // ISS retido pelo tomador
	AliquotaSN           float64 `json:"aliquotaSN,omitempty"`               // alíquota do Simples Nacional (%)
	BaseCalculoPISCOFINS float64 `json:"baseCalculoPisCofins,omitempty"`     // base de PIS/COFINS no layout do provedor
	AliquotaPIS          float64 `json:"aliquotaPis,omitempty"`              // alíquota do PIS (%)
	AliquotaCofins       float64 `json:"aliquotaCofins,omitempty"`           // alíquota da COFINS (%)
	AliquotaINSS         float64 `json:"aliquotaInss,omitempty"`             // alíquota do INSS (%)
	AliquotaIR           float64 `json:"aliquotaIr,omitempty"`               // alíquota do IR (%)
	AliquotaCSLL         float64 `json:"aliquotaCsll,omitempty"`             // alíquota da CSLL (%)
	AliquotaCPP          float64 `json:"aliquotaCpp,omitempty"`              // alíquota da CPP (%)
	ValorCPP             float64 `json:"valorCpp,omitempty"`                 // contribuição previdenciária patronal
	ValorIPI             float64 `json:"valorIpi,omitempty"`                 // valor do IPI
	RetidoPIS            int     `json:"retidoPis,omitempty"`                // 1=Sim, 2=Não
	RetidoCofins         int     `json:"retidoCofins,omitempty"`             // 1=Sim, 2=Não
	RetidoINSS           int     `json:"retidoInss,omitempty"`               // 1=Sim, 2=Não
	RetidoIR             int     `json:"retidoIr,omitempty"`                 // 1=Sim, 2=Não
	RetidoCSLL           int     `json:"retidoCsll,omitempty"`               // 1=Sim, 2=Não
	RetidoCPP            int     `json:"retidoCpp,omitempty"`                // 1=Sim, 2=Não
	RetencoesFederais    float64 `json:"retencoesFederais,omitempty"`        // total das retenções federais
	IrrfIndenizacao      float64 `json:"irrfIndenizacao,omitempty"`          // IRRF sobre indenização
	OutrasRetencoes      float64 `json:"outrasRetencoes,omitempty"`          // outras retenções
	DescricaoOutrasRet   string  `json:"descricaoOutrasRetencoes,omitempty"` // descrição de outras retenções
	OutrosDescontos      float64 `json:"outrosDescontos,omitempty"`          // outros descontos
	JustificativaDeducao string  `json:"justificativaDeducao,omitempty"`     // justificativa da dedução
	ValorRepasse         float64 `json:"valorRepasse,omitempty"`             // valor repassado a terceiros
	ValorInicialCobrado  float64 `json:"valorInicialCobrado,omitempty"`      // valor inicial cobrado
	ValorFinalCobrado    float64 `json:"valorFinalCobrado,omitempty"`        // valor final cobrado
	ValorLiquidoNfse     float64 `json:"valorLiquidoNfse,omitempty"`         // valor líquido da NFS-e
	ValorTotalNotaFiscal float64 `json:"valorTotalNotaFiscal,omitempty"`     // valor total da nota
	ValorTotalRecebido   float64 `json:"valorTotalRecebido,omitempty"`       // valor total recebido
	ValorTotalTributos   float64 `json:"valorTotalTributos,omitempty"`       // valor total dos tributos
	TotalAproxTrib       float64 `json:"totalAproxTrib,omitempty"`           // total aproximado de tributos (Infisc)
	DsImpostos           string  `json:"dsImpostos,omitempty"`               // descrição dos impostos (Equiplano)
}

// TribMun é a tributação municipal (seção [tribMun]).
type TribMun struct {
	TribISSQN   int     `json:"tribISSQN,omitempty" enum:"TributacaoISSQN"` // 1=operação tributável
	TpRetISSQN  int     `json:"tpRetISSQN,omitempty" enum:"TipoRetencaoISSQN"`
	PAliq       float64 `json:"pAliq,omitempty"`       // alíquota (%)
	TpImunidade int     `json:"tpImunidade,omitempty"` // tipo de imunidade
	TpSusp      int     `json:"tpSusp,omitempty"`      // exigibilidade suspensa
	NProcesso   string  `json:"nProcesso,omitempty"`   // processo de suspensão
	VRedBCBM    float64 `json:"vRedBCBM,omitempty"`    // redução da BC (benefício)
	PRedBCBM    float64 `json:"pRedBCBM,omitempty"`
	NBM         string  `json:"nBM,omitempty"`         // nº do benefício municipal
	CPaisResult int     `json:"cPaisResult,omitempty"` // País onde o resultado do serviço aparece, na exportação (BACEN).
}

// TribFed é a tributação federal (seção [tribFederal]): PIS/COFINS + retenções.
type TribFed struct {
	CST            string  `json:"CST,omitempty"`
	VBCPisCofins   float64 `json:"vBCPisCofins,omitempty"`
	PAliqPis       float64 `json:"pAliqPis,omitempty"`
	PAliqCofins    float64 `json:"pAliqCofins,omitempty"`
	VPis           float64 `json:"vPis,omitempty"`
	VCofins        float64 `json:"vCofins,omitempty"`
	TpRetPisCofins *int    `json:"tpRetPisCofins,omitempty"` // tipo de retenção do PIS e da COFINS (0 = PIS/COFINS/CSLL não retidos)
	VRetCP         float64 `json:"vRetCP,omitempty"`
	VRetIRRF       float64 `json:"vRetIRRF,omitempty"`
	VRetCSLL       float64 `json:"vRetCSLL,omitempty"`
	VBCPIRRF       float64 `json:"vBCPIRRF,omitempty"` // Base de cálculo do IRRF.
	VBCCSLL        float64 `json:"vBCCSLL,omitempty"`  // Base de cálculo da CSLL.
	VBCPCP         float64 `json:"vBCPCP,omitempty"`   // Base de cálculo da contribuição previdenciária.
}

// TotTrib são os totais aproximados de tributos (seção [totTrib]).
type TotTrib struct {
	IndTotTrib  *int    `json:"indTotTrib,omitempty"` // indicador de não destaque dos tributos (único valor válido: 0)
	PTotTribSN  float64 `json:"pTotTribSN,omitempty"`
	VTotTribFed float64 `json:"vTotTribFed,omitempty"`
	VTotTribEst float64 `json:"vTotTribEst,omitempty"`
	VTotTribMun float64 `json:"vTotTribMun,omitempty"`
	PTotTribFed float64 `json:"pTotTribFed,omitempty"`
	PTotTribEst float64 `json:"pTotTribEst,omitempty"`
	PTotTribMun float64 `json:"pTotTribMun,omitempty"`
}

// IBSCBSDPS é o grupo da Reforma Tributária no DPS (seção [IBSCBSDPS]).
type IBSCBSDPS struct {
	FinNFSe   string `json:"finNFSe,omitempty"`
	IndFinal  string `json:"indFinal,omitempty"`
	CIndOp    string `json:"cIndOp,omitempty"`
	TpOper    string `json:"tpOper,omitempty"`
	TpEnteGov string `json:"tpEnteGov,omitempty"`
	IndDest   string `json:"indDest,omitempty"`
	// Campos que provedores específicos leem deste grupo: SigISSWeb (operação
	// com o exterior e consumo pessoal), Conam (alíquota única) e eGoverneISS
	// (local de incidência). Todos default "0" na lib, então o que não for
	// informado não muda nada.
	OperExterior      string      `json:"operExterior,omitempty"`
	OperUF            string      `json:"operUF,omitempty"`            // UF da operação com o exterior.
	OperxCidade       string      `json:"operxCidade,omitempty"`       // Cidade da operação com o exterior.
	ConsumoPessoal    string      `json:"consumoPessoal,omitempty"`    // Indica consumo pessoal (0 ou 1).
	IndOpeOne         string      `json:"indOpeOne,omitempty"`         // Indica operação com alíquota única (0 ou 1).
	IdLocalIncidencia string      `json:"idLocalIncidencia,omitempty"` // Local de incidência, no vocabulário do eGoverneISS.
	Dest              *DestIBS    `json:"dest,omitempty"`              // destinatário (seção [Destinatario])
	Imovel            *ImovelIBS  `json:"imovel,omitempty"`            // imóvel (seção [Imovel])
	Documentos        []DocReeRep `json:"documentos,omitempty"`        // gReeRepRes ([DocumentosNNNN])
	GRefNFSe          []string    `json:"gRefNFSe,omitempty"`          // Chaves das NFS-e que este documento referencia ([gRefNFSeNN]).
	GIBSCBS           *GIBSCBSDPS `json:"gIBSCBS,omitempty"`
	// NFSe são os valores de IBS/CBS que parte dos layouts municipais pede
	// DENTRO do RPS (seções [IBSCBSNFSE] e [IBSCBSValoresNFSE] do INI). No
	// Padrão Nacional quem os calcula é o fisco; no GISS 2.04, por exemplo, o
	// gravador os lê daqui, e sem eles a nota sai com cLocalidadeIncid zerado.
	NFSe *IBSCBSNFSe `json:"nfse,omitempty"`
}

// DocReeRep é um documento de reembolso/representação/ressarcimento (gReeRepRes,
// seção [DocumentosNNNN]). Identifica o doc por chave DFe, doc fiscal de outro
// município ou doc avulso, o fornecedor e o tipo/valor de reembolso.
type DocReeRep struct {
	TipoChaveDFe  string  `json:"tipoChaveDFe,omitempty"`
	XTipoChaveDFe string  `json:"xTipoChaveDFe,omitempty"`
	ChaveDFe      string  `json:"chaveDFe,omitempty"`
	CMunDocFiscal string  `json:"cMunDocFiscal,omitempty"`
	NDocFiscal    string  `json:"nDocFiscal,omitempty"`
	XDocFiscal    string  `json:"xDocFiscal,omitempty"`
	NDoc          string  `json:"nDoc,omitempty"`
	XDoc          string  `json:"xDoc,omitempty"`
	FornecCNPJ    string  `json:"fornecCNPJ,omitempty"`
	FornecCPF     string  `json:"fornecCPF,omitempty"`
	FornecNome    string  `json:"fornecNome,omitempty"`
	FornecNIF     string  `json:"fornecNIF,omitempty"`     // Identificação fiscal do fornecedor no exterior.
	FornecCNaoNIF string  `json:"fornecCNaoNIF,omitempty"` // Motivo de não informar o NIF do fornecedor.
	DtEmiDoc      string  `json:"dtEmiDoc"`
	DtCompDoc     string  `json:"dtCompDoc,omitempty"`
	TpReeRepRes   string  `json:"tpReeRepRes,omitempty"`
	XTpReeRepRes  string  `json:"xTpReeRepRes,omitempty"`
	VlrReeRepRes  float64 `json:"vlrReeRepRes,omitempty"`
}

// DestIBS é o destinatário da operação na Reforma (seção [Destinatario]).
type DestIBS struct {
	CNPJ        string `json:"CNPJ,omitempty"`
	CPF         string `json:"CPF,omitempty"`
	NIF         string `json:"NIF,omitempty"`
	XNome       string `json:"xNome,omitempty"`
	IM          string `json:"IM,omitempty"`
	Logradouro  string `json:"logradouro,omitempty"`
	Numero      string `json:"numero,omitempty"`
	Complemento string `json:"complemento,omitempty"`
	Bairro      string `json:"bairro,omitempty"`
	CMun        string `json:"cMun,omitempty"`
	XMun        string `json:"xMun,omitempty"`
	UF          string `json:"UF,omitempty"`
	CEP         string `json:"CEP,omitempty"`
	CPais       string `json:"cPais,omitempty"`
	Email       string `json:"email,omitempty"`
	Telefone    string `json:"telefone,omitempty"`
	IE          string `json:"IE,omitempty"`          // Inscrição estadual do destinatário (SigISSWeb).
	XPais       string `json:"xPais,omitempty"`       // Nome do país, no destinatário do exterior.
	CNaoNIF     string `json:"cNaoNIF,omitempty"`     // Motivo de não informar o NIF.
	TipoServico string `json:"tipoServico,omitempty"` // Tipo de serviço do destinatário (Publica).
}

// ImovelIBS identifica o imóvel da operação (seção [Imovel]).
type ImovelIBS struct {
	InscImobFisc string `json:"inscImobFisc,omitempty"`
	CCIB         string `json:"cCIB,omitempty"`
	Logradouro   string `json:"logradouro,omitempty"`
	Numero       string `json:"numero,omitempty"`
	Complemento  string `json:"complemento,omitempty"`
	Bairro       string `json:"bairro,omitempty"`
	CEP          string `json:"CEP,omitempty"`
	CMun         string `json:"cMun,omitempty"` // Código IBGE do município do imóvel.
	// Imóvel no exterior: o endereço vai nestes campos, e não nos de cima.
	CEndPost    string `json:"cEndPost,omitempty"`
	XCidade     string `json:"xCidade,omitempty"`     // Cidade do imóvel no exterior.
	XEstProvReg string `json:"xEstProvReg,omitempty"` // Estado, província ou região.
	CPais       string `json:"cPais,omitempty"`       // País do imóvel (BACEN).
}

// GIBSCBSDPS é a tributação IBS/CBS (seção [gIBSCBS]).
type GIBSCBSDPS struct {
	CST          string           `json:"CST"`
	CClassTrib   string           `json:"cClassTrib,omitempty"`
	CCredPres    string           `json:"cCredPres,omitempty"`
	GTribRegular *GTribRegularDPS `json:"gTribRegular,omitempty"`
	GDif         *GDifDPS         `json:"gDif,omitempty"`
}

// GTribRegularDPS é a tributação regular de referência (seção [gTribRegular]).
type GTribRegularDPS struct {
	CSTReg        string `json:"CSTReg"`
	CClassTribReg string `json:"cClassTribReg,omitempty"`
}

// GDifDPS é o diferimento por ente (seção [gDif]).
type GDifDPS struct {
	PDifUF  float64 `json:"pDifUF,omitempty"`
	PDifMun float64 `json:"pDifMun,omitempty"`
	PDifCBS float64 `json:"pDifCBS,omitempty"`
}

// ItemServico é um item da lista de serviços do RPS (seção [ItensNNN]).
//
// O contrato tem um serviço único em serv; esta lista existe porque o INI tem
// a dela, e há layout que só monta a nota a partir dos itens. Quando itens vem
// preenchido, é ele que vai ao INI; sem ele, o item é derivado do serviço.
type ItemServico struct {
	Descricao                 string  `json:"descricao"`                            // discriminação do item; é a âncora da lista
	ItemListaServico          string  `json:"itemListaServico,omitempty"`           // item da LC 116
	XItemListaServico         string  `json:"xItemListaServico,omitempty"`          // descrição do item da lista
	CodServico                string  `json:"codServico,omitempty"`                 // código do serviço no provedor
	CodLCServico              string  `json:"codLCServico,omitempty"`               // código da LC do serviço
	CodigoCnae                string  `json:"codigoCnae,omitempty"`                 // CNAE do item
	CodigoTributacaoMunicipio string  `json:"codigoTributacaoMunicipio,omitempty"`  // código de tributação municipal
	XCodigoTribMun            string  `json:"xCodigoTributacaoMunicipio,omitempty"` // descrição do código de tributação
	CodigoServicoNacional     string  `json:"codigoServicoNacional,omitempty"`      // código nacional do serviço
	CodigoTributacaoNacional  string  `json:"codigoTributacaoNacional,omitempty"`   // código de tributação nacional
	CodigoNBS                 string  `json:"codigoNBS,omitempty"`                  // código NBS
	XNBS                      string  `json:"xNBS,omitempty"`                       // descrição do código NBS
	CodigoInterContr          string  `json:"codigoInterContr,omitempty"`           // código interno do contribuinte
	CodCNO                    string  `json:"codCNO,omitempty"`                     // cadastro nacional de obra
	CFPS                      string  `json:"CFPS,omitempty"`                       // código fiscal de prestação de serviço
	CClassTrib                string  `json:"cClassTrib,omitempty"`                 // classificação tributária
	INDOP                     string  `json:"INDOP,omitempty"`                      // indicador de operação
	InfAdicional              string  `json:"infAdicional,omitempty"`               // informação adicional do item
	NumeroProcesso            string  `json:"numeroProcesso,omitempty"`             // processo de suspensão
	IdentifNaoExigib          string  `json:"identifNaoExigibilidade,omitempty"`    // identificador da não exigibilidade
	TipoLancamento            string  `json:"tipoLancamento,omitempty"`             // tipo de lançamento
	Operacao                  string  `json:"operacao,omitempty"`                   // operação do item
	Tributacao                string  `json:"tributacao,omitempty"`                 // tributação do item
	LocalPrestacao            string  `json:"localPrestacao,omitempty"`             // local da prestação
	XFormaPagamento           string  `json:"xFormaPagamento,omitempty"`            // forma de pagamento
	FonteCargaTrib            string  `json:"fonteCargaTributaria,omitempty"`       // fonte da carga tributária
	XJustDeducao              string  `json:"xJustDeducao,omitempty"`               // justificativa da dedução
	DescricaoOutrasRet        string  `json:"descricaoOutrasRetencoes,omitempty"`   // descrição de outras retenções
	Unidade                   string  `json:"unidade,omitempty"`                    // unidade de medida
	TipoUnidade               string  `json:"tipoUnidade,omitempty"`                // tipo da unidade
	XMunicipioIncidencia      string  `json:"xMunicipioIncidencia,omitempty"`       // nome do município de incidência
	CodigoMunicipio           string  `json:"codigoMunicipio,omitempty"`            // município da prestação do item (IBGE)
	MunicipioIncidencia       int     `json:"municipioIncidencia,omitempty"`        // município de incidência do ISS (IBGE)
	CodigoPais                int     `json:"codigoPais,omitempty"`                 // país da prestação (BACEN)
	ExigibilidadeISS          int     `json:"exigibilidadeISS,omitempty"`           // exigibilidade do ISS
	RespRetencao              int     `json:"responsavelRetencao,omitempty"`        // responsável pela retenção
	SituacaoTributaria        int     `json:"situacaoTributaria,omitempty"`         // situação tributária do item
	ISSRetido                 int     `json:"issRetido,omitempty"`                  // 1=Sim, 2=Não
	Tributavel                int     `json:"tributavel,omitempty"`                 // 1=Sim, 2=Não
	TribMunPrestador          int     `json:"tribMunPrestador,omitempty"`           // 1=Sim, 2=Não, tributado no município do prestador
	PrestadoViasPublic        *bool   `json:"prestadoEmViasPublicas,omitempty"`     // prestado em vias públicas
	Quantidade                float64 `json:"quantidade,omitempty"`                 // quantidade
	ValorUnitario             float64 `json:"valorUnitario,omitempty"`              // valor unitário
	ValorTotal                float64 `json:"valorTotal,omitempty"`                 // valor total do item
	ValorServicos             float64 `json:"valorServicos,omitempty"`              // valor dos serviços do item
	QtdeDiaria                float64 `json:"qtdeDiaria,omitempty"`                 // quantidade de diárias
	ValorTaxaTurismo          float64 `json:"valorTaxaTurismo,omitempty"`           // taxa de turismo
	AliquotaDeducoes          float64 `json:"aliquotaDeducoes,omitempty"`           // alíquota de deduções (%)
	ValorDeducoes             float64 `json:"valorDeducoes,omitempty"`              // valor das deduções
	AliqReducao               float64 `json:"aliqReducao,omitempty"`                // alíquota de redução (%)
	ValorReducao              float64 `json:"valorReducao,omitempty"`               // valor da redução
	Aliquota                  float64 `json:"aliquota,omitempty"`                   // alíquota do ISS (%)
	AliquotaSN                float64 `json:"aliquotaSN,omitempty"`                 // alíquota do Simples Nacional (%)
	BaseCalculo               float64 `json:"baseCalculo,omitempty"`                // base de cálculo do ISS
	ValorISS                  float64 `json:"valorISS,omitempty"`                   // ISS do item
	ValorISSRetido            float64 `json:"valorISSRetido,omitempty"`             // ISS retido do item
	AliqISSST                 float64 `json:"aliqISSST,omitempty"`                  // alíquota do ISS substituição (%)
	ValorISSST                float64 `json:"valorISSST,omitempty"`                 // ISS por substituição
	ValorTributavel           float64 `json:"valorTributavel,omitempty"`            // valor tributável
	DescontoIncondicionado    float64 `json:"descontoIncondicionado,omitempty"`     // desconto incondicionado
	DescontoCondicionado      float64 `json:"descontoCondicionado,omitempty"`       // desconto condicionado
	OutrosDescontos           float64 `json:"outrosDescontos,omitempty"`            // outros descontos
	OutrasRetencoes           float64 `json:"outrasRetencoes,omitempty"`            // outras retenções
	RetencoesFederais         float64 `json:"retencoesFederais,omitempty"`          // total das retenções federais
	IrrfIndenizacao           float64 `json:"irrfIndenizacao,omitempty"`            // IRRF sobre indenização
	ValorBCCSLL               float64 `json:"valorBCCSLL,omitempty"`                // base da CSLL
	AliqRetCSLL               float64 `json:"aliqRetCSLL,omitempty"`                // alíquota de retenção da CSLL (%)
	RetidoCSLL                int     `json:"retidoCSLL,omitempty"`                 // 1=Sim, 2=Não
	ValorCSLL                 float64 `json:"valorCSLL,omitempty"`                  // valor da CSLL
	ValorBCPIS                float64 `json:"valorBCPIS,omitempty"`                 // base do PIS
	AliqRetPIS                float64 `json:"aliqRetPIS,omitempty"`                 // alíquota de retenção do PIS (%)
	RetidoPIS                 int     `json:"retidoPIS,omitempty"`                  // 1=Sim, 2=Não
	ValorPIS                  float64 `json:"valorPIS,omitempty"`                   // valor do PIS
	ValorBCCOFINS             float64 `json:"valorBCCOFINS,omitempty"`              // base da COFINS
	AliqRetCOFINS             float64 `json:"aliqRetCOFINS,omitempty"`              // alíquota de retenção da COFINS (%)
	RetidoCOFINS              int     `json:"retidoCOFINS,omitempty"`               // 1=Sim, 2=Não
	ValorCOFINS               float64 `json:"valorCOFINS,omitempty"`                // valor da COFINS
	ValorBCINSS               float64 `json:"valorBCINSS,omitempty"`                // base do INSS
	AliqRetINSS               float64 `json:"aliqRetINSS,omitempty"`                // alíquota de retenção do INSS (%)
	RetidoINSS                int     `json:"retidoINSS,omitempty"`                 // 1=Sim, 2=Não
	ValorINSS                 float64 `json:"valorINSS,omitempty"`                  // valor do INSS
	ValorBCRetIRRF            float64 `json:"valorBCRetIRRF,omitempty"`             // base do IRRF
	AliqRetIRRF               float64 `json:"aliqRetIRRF,omitempty"`                // alíquota de retenção do IRRF (%)
	RetidoIRRF                int     `json:"retidoIRRF,omitempty"`                 // 1=Sim, 2=Não
	ValorIRRF                 float64 `json:"valorIRRF,omitempty"`                  // valor do IRRF
	ValorBCCPP                float64 `json:"valorBCCPP,omitempty"`                 // base da CPP
	AliqRetCPP                float64 `json:"aliqRetCPP,omitempty"`                 // alíquota de retenção da CPP (%)
	RetidoCPP                 int     `json:"retidoCPP,omitempty"`                  // 1=Sim, 2=Não
	ValorCPP                  float64 `json:"valorCPP,omitempty"`                   // valor da CPP
	ValorIPI                  float64 `json:"valorIPI,omitempty"`                   // valor do IPI
	ValorRecebido             float64 `json:"valorRecebido,omitempty"`              // valor recebido do item
	ValorLiquidoNfse          float64 `json:"valorLiquidoNfse,omitempty"`           // valor líquido do item
	ValorRepasse              float64 `json:"valorRepasse,omitempty"`               // valor repassado a terceiros
	ValorInicialCobrado       float64 `json:"valorInicialCobrado,omitempty"`        // valor inicial cobrado
	ValorFinalCobrado         float64 `json:"valorFinalCobrado,omitempty"`          // valor final cobrado
	PercCargaTrib             float64 `json:"percentualCargaTributaria,omitempty"`  // carga tributária do item (%)
	ValorCargaTrib            float64 `json:"valorCargaTributaria,omitempty"`       // carga tributária do item
	TotalAproxTribServ        float64 `json:"totalAproxTribServ,omitempty"`         // total aproximado de tributos do serviço
	AliqIBS                   float64 `json:"aliqIBS,omitempty"`                    // alíquota do IBS (%)
	AliqCBS                   float64 `json:"aliqCBS,omitempty"`                    // alíquota da CBS (%)
	// EnderecoServico é o endereço de execução DESTE item ([EnderecoServicoNNN],
	// com o mesmo índice do item).
	EnderecoServico *EnderecoServico `json:"enderecoServico,omitempty"`
}

// ConstrucaoCivil é a obra a que o serviço se refere (seção
// [ConstrucaoCivil]). O endereço da obra vai achatado na mesma seção.
type ConstrucaoCivil struct {
	CodigoObra            string `json:"codigoObra,omitempty"`            // código da obra (CNO)
	InscImobFisc          string `json:"inscImobFisc,omitempty"`          // inscrição imobiliária fiscal
	Cib                   int    `json:"cib,omitempty"`                   // cadastro imobiliário brasileiro
	Art                   string `json:"art,omitempty"`                   // anotação de responsabilidade técnica
	ObrasOpcao            int    `json:"obrasOpcao,omitempty"`            // opção de informação da obra
	LocalConstrucao       string `json:"localConstrucao,omitempty"`       // local da construção
	ReformaCivil          int    `json:"reformaCivil,omitempty"`          // 1=Sim, 2=Não
	NCei                  string `json:"nCei,omitempty"`                  // número do CEI da obra
	NProj                 string `json:"nProj,omitempty"`                 // número do projeto
	NMatri                string `json:"nMatri,omitempty"`                // número da matrícula
	NNumeroEncapsulamento string `json:"nNumeroEncapsulamento,omitempty"` // número de encapsulamento
	Logradouro            string `json:"logradouro,omitempty"`            // logradouro da obra
	Numero                string `json:"numero,omitempty"`                // número
	Complemento           string `json:"complemento,omitempty"`           // complemento
	Bairro                string `json:"bairro,omitempty"`                // bairro
	CEP                   string `json:"CEP,omitempty"`                   // CEP
	CodigoMunicipio       string `json:"codigoMunicipio,omitempty"`       // município da obra (IBGE)
	XMunicipio            string `json:"xMunicipio,omitempty"`            // nome do município
	UF                    string `json:"UF,omitempty"`                    // UF
}

// DocDeducaoReducao é um documento que embasa dedução ou redução da base
// (seção [DocumentosDeducaoReducaoNNNN], com o fornecedor em [FornecedorNNNN]).
type DocDeducaoReducao struct {
	DtEmiDoc            string      `json:"dtEmiDoc" fmt:"data"`           // emissão do documento
	TpDedRed            string      `json:"tpDedRed,omitempty"`            // tipo de dedução ou redução
	XDescOutDed         string      `json:"xDescOutDed,omitempty"`         // descrição de outra dedução
	VDedutivelRedutivel float64     `json:"vDedutivelRedutivel,omitempty"` // valor dedutível ou redutível
	VDeducaoReducao     float64     `json:"vDeducaoReducao,omitempty"`     // valor deduzido ou reduzido
	ChNFSe              string      `json:"chNFSe,omitempty"`              // chave da NFS-e nacional
	ChNFe               string      `json:"chNFe,omitempty"`               // chave da NF-e
	NDocFisc            string      `json:"nDocFisc,omitempty"`            // número do documento fiscal
	NDoc                string      `json:"nDoc,omitempty"`                // número do documento não fiscal
	CMunNFSeMun         string      `json:"cMunNFSeMun,omitempty"`         // município da NFS-e municipal
	NNFSeMun            string      `json:"nNFSeMun,omitempty"`            // número da NFS-e municipal
	CVerifNFSeMun       string      `json:"cVerifNFSeMun,omitempty"`       // código de verificação da NFS-e municipal
	NNFS                string      `json:"nNFS,omitempty"`                // número da nota fiscal de serviço
	ModNFS              string      `json:"modNFS,omitempty"`              // modelo da nota fiscal de serviço
	SerieNFS            string      `json:"serieNFS,omitempty"`            // série da nota fiscal de serviço
	Fornecedor          *Fornecedor `json:"fornecedor,omitempty"`          // quem emitiu o documento
}

// Fornecedor é quem emitiu o documento de dedução (seção [FornecedorNNNN]). Tem
// struct própria porque a seção lê menos campos que uma pessoa do documento:
// mandar o resto viraria chave morta.
type Fornecedor struct {
	CNPJCPF            string `json:"CNPJCPF,omitempty"`            // CNPJ ou CPF
	InscricaoMunicipal string `json:"inscricaoMunicipal,omitempty"` // inscrição municipal
	NIF                string `json:"NIF,omitempty"`                // identificação fiscal no exterior
	CNaoNIF            string `json:"cNaoNIF,omitempty"`            // motivo de não informar o NIF
	CAEPF              string `json:"CAEPF,omitempty"`              // cadastro de atividade econômica de pessoa física
	RazaoSocial        string `json:"razaoSocial,omitempty"`        // razão social
	Logradouro         string `json:"logradouro,omitempty"`         // logradouro
	Numero             string `json:"numero,omitempty"`             // número
	Complemento        string `json:"complemento,omitempty"`        // complemento
	Bairro             string `json:"bairro,omitempty"`             // bairro
	CEP                string `json:"CEP,omitempty"`                // CEP
	XMunicipio         string `json:"xMunicipio,omitempty"`         // nome do município
	UF                 string `json:"UF,omitempty"`                 // UF
	Telefone           string `json:"telefone,omitempty"`           // telefone
	Email              string `json:"email,omitempty"`              // e-mail
}

// EnderecoServico é o endereço de execução do serviço (seção
// [EnderecoServicoNNN], por item do RPS).
type EnderecoServico struct {
	EnderecoInformado string `json:"enderecoInformado,omitempty"` // 1=Sim, 2=Não, endereço informado
	TipoLogradouro    string `json:"tipoLogradouro,omitempty"`    // tipo do logradouro (Rua, Avenida)
	Endereco          string `json:"endereco,omitempty"`          // logradouro
	Numero            string `json:"numero,omitempty"`            // número
	Complemento       string `json:"complemento,omitempty"`       // complemento
	TipoBairro        string `json:"tipoBairro,omitempty"`        // tipo do bairro
	Bairro            string `json:"bairro,omitempty"`            // bairro
	CodigoMunicipio   string `json:"codigoMunicipio,omitempty"`   // município (IBGE)
	UF                string `json:"UF,omitempty"`                // UF
	CEP               string `json:"CEP,omitempty"`               // CEP
	XMunicipio        string `json:"xMunicipio,omitempty"`        // nome do município
	CodigoPais        int    `json:"codigoPais,omitempty"`        // país (BACEN; 1058 = Brasil)
	XPais             string `json:"xPais,omitempty"`             // nome do país
	PontoReferencia   string `json:"pontoReferencia,omitempty"`   // ponto de referência
}

// CondicaoPagamento é o parcelamento do documento (seção [CondicaoPagamento] e
// as parcelas em [ParcelasNN]).
type CondicaoPagamento struct {
	QtdParcela         int       `json:"qtdParcela,omitempty"`                // quantidade de parcelas
	Condicao           string    `json:"condicao,omitempty"`                  // condição de pagamento
	DataCriacao        string    `json:"dataCriacao,omitempty" fmt:"data"`    // data de criação
	DataVencimento     string    `json:"dataVencimento,omitempty" fmt:"data"` // vencimento
	InstrucaoPagamento string    `json:"instrucaoPagamento,omitempty"`        // instrução de pagamento
	CodigoVencimento   string    `json:"codigoVencimento,omitempty"`          // código de vencimento do provedor
	Parcelas           []Parcela `json:"parcelas,omitempty"`                  // parcelas do parcelamento
}

// Parcela é uma parcela do parcelamento (seção [ParcelasNN]).
type Parcela struct {
	Parcela        string  `json:"parcela"`                             // identificação da parcela
	DataVencimento string  `json:"dataVencimento,omitempty" fmt:"data"` // vencimento da parcela
	Valor          float64 `json:"valor,omitempty"`                     // valor da parcela
	Condicao       string  `json:"condicao,omitempty"`                  // condição de pagamento da parcela
}

// Transportadora identifica quem transporta (seção [Transportadora]).
type Transportadora struct {
	XNome       string  `json:"xNomeTrans,omitempty"`      // nome da transportadora
	CpfCnpj     string  `json:"xCpfCnpjTrans,omitempty"`   // CNPJ ou CPF da transportadora
	InscEstuary string  `json:"xInscEstTrans,omitempty"`   // inscrição estadual
	Placa       string  `json:"xPlacaTrans,omitempty"`     // placa do veículo
	Endereco    string  `json:"xEndTrans,omitempty"`       // endereço
	CMun        string  `json:"cMunTrans,omitempty"`       // município (IBGE)
	XMun        string  `json:"xMunTrans,omitempty"`       // nome do município
	UF          string  `json:"xUFTrans,omitempty"`        // UF
	XPais       string  `json:"xPaisTrans,omitempty"`      // país
	TipoFrete   float64 `json:"vTipoFreteTrans,omitempty"` // tipo de frete
}

// Quarto é a diária de hotelaria (seção [QuartosNNN]).
type Quarto struct {
	CodigoInterno int     `json:"codigoInternoQuarto"`          // código interno do quarto
	QtdHospedes   int     `json:"qtdHospedes,omitempty"`        // quantidade de hóspedes
	CheckIn       string  `json:"checkIn,omitempty" fmt:"data"` // data do check-in
	QtdDiarias    int     `json:"qtdDiarias,omitempty"`         // quantidade de diárias
	ValorDiaria   float64 `json:"valorDiaria,omitempty"`        // valor da diária
}

// Despesa é uma despesa reembolsável (seção [DespesasNNN]).
type Despesa struct {
	NItem string  `json:"nItemDesp,omitempty"`        // número do item da despesa
	XDesp string  `json:"xDesp,omitempty"`            // descrição da despesa
	DDesp string  `json:"dDesp,omitempty" fmt:"data"` // data da despesa
	VDesp float64 `json:"vDesp"`                      // valor da despesa
}

// Generico é um campo livre do provedor (seção [GenericosN]).
type Generico struct {
	Titulo    string `json:"titulo"`              // título do campo
	Descricao string `json:"descricao,omitempty"` // conteúdo do campo
}

// Imposto é a quebra de imposto do serviço (seção [ImpostosNNN]).
type Imposto struct {
	Codigo    int     `json:"codigo,omitempty"`    // código do imposto
	Descricao string  `json:"descricao,omitempty"` // descrição do imposto
	Aliquota  float64 `json:"aliquota,omitempty"`  // alíquota (%)
	Valor     float64 `json:"valor"`               // valor do imposto
}

// Deducao é uma dedução por documento referenciado (seção [DeducoesNNN]).
type Deducao struct {
	TipoDeducao          string  `json:"tipoDeducao,omitempty"`          // tipo da dedução
	CpfCnpjReferencia    string  `json:"CNPJCPF,omitempty"`              // documento do referenciado
	NumeroNFReferencia   string  `json:"numeroNFReferencia,omitempty"`   // número da nota referenciada
	ValorTotalReferencia float64 `json:"valorTotalReferencia,omitempty"` // valor total da nota referenciada
	PercentualDeduzir    float64 `json:"percentualDeduzir,omitempty"`    // percentual a deduzir
	ValorDeduzir         float64 `json:"valorDeduzir"`                   // valor a deduzir
	DeducaoPor           string  `json:"deducaoPor,omitempty"`           // dedução por valor ou percentual
}

// EventoServico é a atividade de evento a que o serviço se refere (show,
// feira, congresso), na seção [Evento] do INI.
//
// Não confundir com EventoPN, que é o pedido de registro de evento. São INIs
// diferentes, de chamadas diferentes, que a biblioteca nomeia igual: o leitor
// da nota lê esta seção, e o leitor de pedidos lê a outra.
type EventoServico struct {
	XNome       string `json:"xNome,omitempty"`                // Nome do evento.
	DtIni       string `json:"dtIni,omitempty" fmt:"data"`     // Data de início do evento.
	DtFim       string `json:"dtFim,omitempty" fmt:"data"`     // Data de encerramento do evento.
	IdAtvEvt    string `json:"idAtvEvt,omitempty"`             // Identificador da atividade do evento.
	Opcao       int    `json:"atividadeEventoOpcao,omitempty"` // Opção da atividade, no layout do provedor.
	CEP         string `json:"CEP,omitempty"`                  // CEP do local do evento.
	XMunicipio  string `json:"xMunicipio,omitempty"`           // Nome do município do evento.
	CMun        string `json:"cMun,omitempty"`                 // Código IBGE do município do evento.
	UF          string `json:"UF,omitempty"`                   // UF do evento.
	Logradouro  string `json:"logradouro,omitempty"`
	Numero      string `json:"numero,omitempty"`
	Complemento string `json:"complemento,omitempty"`
	Bairro      string `json:"bairro,omitempty"`
}

// RpsSubstituido identifica o RPS que este documento substitui (seção
// [RpsSubstituido]). É o caminho do ABRASF: no Padrão Nacional a substituição
// é por chave, e quem a faz é a emissão da nota nova (ver ToINISubstituicaoPN).
type RpsSubstituido struct {
	Numero string `json:"numero,omitempty"`
	Serie  string `json:"serie,omitempty"`
	Tipo   string `json:"tipo,omitempty"` // Tipo do RPS substituído (1 = RPS).
}

// Locacao é a locação e sublocação de postes e dutos (seção
// [LocacaoSubLocacao]).
type Locacao struct {
	Categ    string  `json:"categ,omitempty"`    // categoria da locação
	Objeto   string  `json:"objeto,omitempty"`   // objeto locado
	Extensao float64 `json:"extensao,omitempty"` // extensão
	NPostes  int     `json:"nPostes,omitempty"`  // quantidade de postes
}

// Rodoviaria é a exploração de rodovia (seção [Rodoviaria]).
type Rodoviaria struct {
	CategVeic    string `json:"categVeic,omitempty"`    // categoria do veículo
	NEixos       int    `json:"nEixos,omitempty"`       // quantidade de eixos
	Rodagem      string `json:"rodagem,omitempty"`      // tipo de rodagem
	Sentido      string `json:"sentido,omitempty"`      // sentido da via
	Placa        string `json:"placa,omitempty"`        // placa do veículo
	CodAcessoPed string `json:"codAcessoPed,omitempty"` // código de acesso ao pedágio
	CodContrato  string `json:"codContrato,omitempty"`  // código do contrato
}

// IBSCBSNFSe são os valores de IBS/CBS do lado da NFS-e (seção [IBSCBSNFSE]).
type IBSCBSNFSe struct {
	CLocalidadeIncid string             `json:"cLocalidadeIncid,omitempty"` // município de incidência (IBGE)
	XLocalidadeIncid string             `json:"xLocalidadeIncid,omitempty"` // nome da localidade de incidência
	PRedutor         float64            `json:"pRedutor,omitempty"`         // redutor da base (%)
	Valores          *IBSCBSValoresNFSe `json:"valores,omitempty"`
	TotCIBS          *TotCIBS           `json:"totCIBS,omitempty"` // Totais consolidados de IBS/CBS ([TotCIBS]).
}

// IBSCBSValoresNFSe são a base e as alíquotas de IBS/CBS (seção
// [IBSCBSValoresNFSE]): estadual (UF), municipal (Mun) e federal (CBS).
type IBSCBSValoresNFSe struct {
	VBC            float64 `json:"vBC,omitempty"`            // base de cálculo
	VCalcReeRepRes float64 `json:"vCalcReeRepRes,omitempty"` // base de reembolso/repasse/ressarcimento
	PIBSUF         float64 `json:"pIBSUF,omitempty"`         // alíquota do IBS estadual (%)
	PRedAliqUF     float64 `json:"pRedAliqUF,omitempty"`     // redução da alíquota estadual (%)
	PAliqEfetUF    float64 `json:"pAliqEfetUF,omitempty"`    // alíquota efetiva estadual (%)
	PIBSMun        float64 `json:"pIBSMun,omitempty"`        // alíquota do IBS municipal (%)
	PRedAliqMun    float64 `json:"pRedAliqMun,omitempty"`    // redução da alíquota municipal (%)
	PAliqEfetMun   float64 `json:"pAliqEfetMun,omitempty"`   // alíquota efetiva municipal (%)
	PCBS           float64 `json:"pCBS,omitempty"`           // alíquota da CBS (%)
	PRedAliqCBS    float64 `json:"pRedAliqCBS,omitempty"`    // redução da alíquota da CBS (%)
	PAliqEfetCBS   float64 `json:"pAliqEfetCBS,omitempty"`   // alíquota efetiva da CBS (%)
}

// TotCIBS são os totais consolidados de IBS/CBS (seção [TotCIBS] e as quatro
// que ela abre). Ficam do lado da NFS-e, não da DPS: quem os calcula é o fisco.
//
// Estão no contrato pelo mesmo motivo de [IBSCBSNFSE]: o leitor os alcança a
// partir do RPS, e há gravador municipal que os lê de lá. Sem a seção [TotCIBS]
// o leitor nem entra nas outras quatro, então elas vão aninhadas aqui.
type TotCIBS struct {
	VTotNF      float64           `json:"vTotNF,omitempty"`         // Valor total da nota.
	TribRegular *GTribRegularNFSe `json:"gTribRegular,omitempty"`   // Tributação regular de referência ([gTribRegularNFSe]).
	CompraGov   *GTribCompraGov   `json:"gTribCompraGov,omitempty"` // Compra governamental ([gTribCompraGov]).
	IBS         *TotgIBS          `json:"gIBS,omitempty"`           // Totais do IBS ([TotgIBS]).
	CBS         *TotgCBS          `json:"gCBS,omitempty"`           // Totais da CBS ([TotgCBS]).
}

// GTribRegularNFSe é a tributação regular de referência da NFS-e (seção
// [gTribRegularNFSe]): o que seria devido fora do regime diferenciado. Não
// confundir com GTribRegularDPS, que é o grupo equivalente da DPS.
type GTribRegularNFSe struct {
	PAliqEfeRegIBSUF  float64 `json:"pAliqEfeRegIBSUF,omitempty"`  // Alíquota efetiva regular do IBS estadual (%).
	VTribRegIBSUF     float64 `json:"vTribRegIBSUF,omitempty"`     // Tributo regular do IBS estadual.
	PAliqEfeRegIBSMun float64 `json:"pAliqEfeRegIBSMun,omitempty"` // Alíquota efetiva regular do IBS municipal (%).
	VTribRegIBSMun    float64 `json:"vTribRegIBSMun,omitempty"`    // Tributo regular do IBS municipal.
	PAliqEfeRegCBS    float64 `json:"pAliqEfeRegCBS,omitempty"`    // Alíquota efetiva regular da CBS (%).
	VTribRegCBS       float64 `json:"vTribRegCBS,omitempty"`       // Tributo regular da CBS.
}

// GTribCompraGov é a tributação da compra governamental (seção
// [gTribCompraGov]).
type GTribCompraGov struct {
	PIBSUF  float64 `json:"pIBSUF,omitempty"`  // Alíquota do IBS estadual na compra governamental (%).
	VIBSUF  float64 `json:"vIBSUF,omitempty"`  // Valor do IBS estadual.
	PIBSMun float64 `json:"pIBSMun,omitempty"` // Alíquota do IBS municipal (%).
	VIBSMun float64 `json:"vIBSMun,omitempty"` // Valor do IBS municipal.
	PCBS    float64 `json:"pCBS,omitempty"`    // Alíquota da CBS (%).
	VCBS    float64 `json:"vCBS,omitempty"`    // Valor da CBS.
}

// TotgIBS são os totais do IBS (seção [TotgIBS]): o crédito presumido e as
// parcelas estadual e municipal.
type TotgIBS struct {
	VIBSTot      float64 `json:"vIBSTot,omitempty"`      // Total do IBS.
	PCredPresIBS float64 `json:"pCredPresIBS,omitempty"` // Percentual do crédito presumido do IBS (%).
	VCredPresIBS float64 `json:"vCredPresIBS,omitempty"` // Valor do crédito presumido do IBS.
	VDifUF       float64 `json:"vDifUF,omitempty"`       // Diferimento do IBS estadual.
	VIBSUF       float64 `json:"vIBSUF,omitempty"`       // Valor do IBS estadual.
	VDifMun      float64 `json:"vDifMun,omitempty"`      // Diferimento do IBS municipal.
	VIBSMun      float64 `json:"vIBSMun,omitempty"`      // Valor do IBS municipal.
}

// TotgCBS são os totais da CBS (seção [TotgCBS]).
type TotgCBS struct {
	VDifCBS      float64 `json:"vDifCBS,omitempty"`      // Diferimento da CBS.
	VCBS         float64 `json:"vCBS,omitempty"`         // Valor da CBS.
	PCredPresCBS float64 `json:"pCredPresCBS,omitempty"` // Percentual do crédito presumido da CBS (%).
	VCredPresCBS float64 `json:"vCredPresCBS,omitempty"` // Valor do crédito presumido da CBS.
}
