// Package nfse modela a NFS-e (Padrão Nacional) com nomes alinhados ao
// contrato da Nuvem Fiscal (infDPS: prest/toma/serv/valores) e traduz esse
// JSON para o arquivo INI consumido pela ACBrLibNFSe.
//
// Este é um SUBCONJUNTO dos campos do DPS nacional: cobre uma emissão básica
// e será expandido (idealmente gerando os tipos a partir do OpenAPI da Nuvem
// Fiscal) nas próximas iterações.
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
	Ambiente   string `json:"ambiente"`             // "homologacao" | "producao"
	Referencia string `json:"referencia,omitempty"` // id externo do cliente
	InfDPS     InfDPS `json:"infDPS"`
}

// InfDPS são as informações da DPS (Declaração de Prestação de Serviços).
type InfDPS struct {
	Serie    string     `json:"serie"`
	NDPS     string     `json:"nDPS"`                                 // número do RPS/DPS
	DCompet  string     `json:"dCompet" fmt:"data"`                   // competência (YYYY-MM-DD)
	DhEmi    string     `json:"dhEmi,omitempty" fmt:"data"`           // emissão; vazio = hoje
	TpEmit   int        `json:"tpEmit,omitempty" enum:"TipoEmitente"` // 1 = prestador
	VerAplic string     `json:"verAplic,omitempty"`
	CLocEmi  string     `json:"cLocEmi,omitempty"` // município emissor (IBGE); vazio = cMun do prestador
	Prest    Pessoa     `json:"prest"`
	Toma     *Pessoa    `json:"toma,omitempty"`
	Interm   *Pessoa    `json:"interm,omitempty"` // intermediário (seção [Intermediario])
	Serv     Servico    `json:"serv"`
	Valores  Valores    `json:"valores"`
	IBSCBS   *IBSCBSDPS `json:"ibscbs,omitempty"` // Reforma Tributária (grupo IBSCBSDPS)
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
	// CPais/XPais só precisam ser informados para endereço NO EXTERIOR. Endereço
	// nacional (com cMun) recebe 1058 (Brasil) automaticamente: ver pessoaCommon.
	CPais int    `json:"cPais,omitempty"` // código do país (1058 = Brasil)
	XPais string `json:"xPais,omitempty"` // nome do país
	// RegTrib (regime tributário) é obrigatório para o prestador no Padrão
	// Nacional, sem ele o ACBr falha ao montar o XML.
	RegTrib *RegTrib `json:"regTrib,omitempty"`
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
	CMunPrestacao  string     `json:"cMunPrestacao,omitempty"`       // município da prestação (IBGE)
	CServ          string     `json:"cServ,omitempty"`               // item da lista (→ cTribNac no Padrão Nacional)
	CTribMun       string     `json:"cTribMun,omitempty"`            // código de tributação municipal
	CNBS           string     `json:"cNBS,omitempty"`                // código NBS
	CodigoCnae     string     `json:"codigoCnae,omitempty"`          // CNAE
	XDescServ      string     `json:"xDescServ"`                     // discriminação
	ExigibISS      int        `json:"exigibilidadeISS,omitempty"`    // 1=exigível, 2=não incidência...
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
}

// TribMun é a tributação municipal (seção [tribMun]).
type TribMun struct {
	TribISSQN   int     `json:"tribISSQN,omitempty" enum:"TributacaoISSQN"` // 1=operação tributável
	TpRetISSQN  int     `json:"tpRetISSQN,omitempty"`                       // 1=não retido, 2=retido tomador...
	PAliq       float64 `json:"pAliq,omitempty"`                            // alíquota (%)
	TpImunidade int     `json:"tpImunidade,omitempty"`                      // tipo de imunidade
	TpSusp      int     `json:"tpSusp,omitempty"`                           // exigibilidade suspensa
	NProcesso   string  `json:"nProcesso,omitempty"`                        // processo de suspensão
	VRedBCBM    float64 `json:"vRedBCBM,omitempty"`                         // redução da BC (benefício)
	PRedBCBM    float64 `json:"pRedBCBM,omitempty"`
	NBM         string  `json:"nBM,omitempty"` // nº do benefício municipal
}

// TribFed é a tributação federal (seção [tribFed]): PIS/COFINS + retenções.
type TribFed struct {
	CST            string  `json:"CST,omitempty"`
	VBCPisCofins   float64 `json:"vBCPisCofins,omitempty"`
	PAliqPis       float64 `json:"pAliqPis,omitempty"`
	PAliqCofins    float64 `json:"pAliqCofins,omitempty"`
	VPis           float64 `json:"vPis,omitempty"`
	VCofins        float64 `json:"vCofins,omitempty"`
	TpRetPisCofins int     `json:"tpRetPisCofins,omitempty"`
	VRetCP         float64 `json:"vRetCP,omitempty"`
	VRetIRRF       float64 `json:"vRetIRRF,omitempty"`
	VRetCSLL       float64 `json:"vRetCSLL,omitempty"`
}

// TotTrib são os totais aproximados de tributos (seção [totTrib]).
type TotTrib struct {
	IndTotTrib  int     `json:"indTotTrib,omitempty"`
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
	FinNFSe    string      `json:"finNFSe,omitempty"`
	IndFinal   string      `json:"indFinal,omitempty"`
	CIndOp     string      `json:"cIndOp,omitempty"`
	TpOper     string      `json:"tpOper,omitempty"`
	TpEnteGov  string      `json:"tpEnteGov,omitempty"`
	IndDest    string      `json:"indDest,omitempty"`
	Dest       *DestIBS    `json:"dest,omitempty"`       // destinatário (seção [Destinatario])
	Imovel     *ImovelIBS  `json:"imovel,omitempty"`     // imóvel (seção [Imovel])
	Documentos []DocReeRep `json:"documentos,omitempty"` // gReeRepRes ([DocumentosNNNN])
	GIBSCBS    *GIBSCBSDPS `json:"gIBSCBS,omitempty"`
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
