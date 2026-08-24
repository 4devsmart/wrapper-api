// Package mdfe traduz o contrato JSON do MDF-e (modelo 58): espelhando a
// ACBr.API (MdfePedidoEmissao = layout SEFAZ COMPLETO): para o INI consumido
// pela ACBrLibMDFe (MDFE_CarregarINI), e interpreta as respostas.
//
// O escopo é o MODAL RODOVIÁRIO. Todo campo declarado aqui ou chega ao arquivo
// intermediário, ou está registrado em testdata/nao_espelhadas.tsv com o
// motivo: o teste de espelho (espelho_test.go) preenche o modelo inteiro e
// cobra cada folha na saída, então "existe no payload e não sei se sai" deixou
// de ser um estado possível. Grupos opcionais podem ser omitidos (omitempty)
// sem erro de consumo.
package mdfe

import "log/slog"

// PedidoEmissao é o MDF-e a ser gerado, no contrato da Nuvem Fiscal / ACBr.API.
type PedidoEmissao struct {
	InfMDFe     InfMDFe      `json:"infMDFe"`
	InfMDFeSupl *InfMDFeSupl `json:"infMDFeSupl,omitempty"`
	Ambiente    string       `json:"ambiente" enum:"Ambiente"`
	Referencia  string       `json:"referencia,omitempty"`
}

// InfMDFe espelha MdfeSefazInfMDFe.
type InfMDFe struct {
	Versao      string       `json:"versao"`
	Id          string       `json:"Id,omitempty"`
	Ide         Ide          `json:"ide"`
	Emit        Emit         `json:"emit"`
	InfModal    InfModal     `json:"infModal"`
	InfDoc      InfDoc       `json:"infDoc"`
	Seg         []Seg        `json:"seg,omitempty"`
	ProdPred    *ProdPred    `json:"prodPred,omitempty"`
	Tot         Tot          `json:"tot"`
	Lacres      []Lacres     `json:"lacres,omitempty"`
	AutXML      []AutXML     `json:"autXML,omitempty"`
	InfAdic     *InfAdic     `json:"infAdic,omitempty"`
	InfRespTec  *RespTec     `json:"infRespTec,omitempty"`
	InfSolicNFF *InfSolicNFF `json:"infSolicNFF,omitempty"`
}

// Ide espelha MdfeSefazIde.
type Ide struct {
	CUF int `json:"cUF"`
	// Redundante com o campo ambiente do pedido, e mantido para quem já monta o
	// documento completo: informado, precisa CONCORDAR com ele, senão a
	// requisição é recusada. Quem manda no XML e na sessão é o ambiente.
	TpAmb               int             `json:"tpAmb,omitempty" enum:"TipoAmbiente"`
	TpEmit              int             `json:"tpEmit" enum:"TipoEmitente"`
	TpTransp            int             `json:"tpTransp,omitempty" enum:"TipoTransportador"`
	Mod                 int             `json:"mod,omitempty" enum:"Modelo"`
	Serie               int             `json:"serie"`
	NMDF                int             `json:"nMDF"`
	CMDF                string          `json:"cMDF,omitempty"`
	CDV                 int             `json:"cDV,omitempty"`
	Modal               int             `json:"modal" enum:"Modal"`
	DhEmi               string          `json:"dhEmi" fmt:"data-hora"`
	TpEmis              int             `json:"tpEmis" enum:"TipoEmissao"`
	ProcEmi             string          `json:"procEmi" enum:"ProcessoEmissao"`
	VerProc             string          `json:"verProc"`
	UFIni               string          `json:"UFIni"`
	UFFim               string          `json:"UFFim"`
	InfMunCarrega       []InfMunCarrega `json:"infMunCarrega"`
	InfPercurso         []InfPercurso   `json:"infPercurso,omitempty"`
	DhIniViagem         string          `json:"dhIniViagem,omitempty" fmt:"data-hora"`
	IndCanalVerde       int             `json:"indCanalVerde,omitempty" enum:"Sim"`
	IndCarregaPosterior int             `json:"indCarregaPosterior,omitempty" enum:"Sim"`
}

// InfMunCarrega espelha MdfeSefazInfMunCarrega.
type InfMunCarrega struct {
	CMunCarrega string `json:"cMunCarrega"`
	XMunCarrega string `json:"xMunCarrega"`
}

// InfPercurso espelha MdfeSefazInfPercurso.
type InfPercurso struct {
	UFPer string `json:"UFPer"`
}

// Emit espelha MdfeSefazEmit.
type Emit struct {
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	XNome     string   `json:"xNome,omitempty"`
	XFant     string   `json:"xFant,omitempty"`
	EnderEmit *EndeEmi `json:"enderEmit,omitempty"`
}

// EndeEmi espelha MdfeSefazEndeEmi.
type EndeEmi struct {
	XLgr    string `json:"xLgr,omitempty"`
	Nro     string `json:"nro,omitempty"`
	XCpl    string `json:"xCpl,omitempty"`
	XBairro string `json:"xBairro,omitempty"`
	CMun    string `json:"cMun,omitempty"`
	XMun    string `json:"xMun,omitempty"`
	CEP     string `json:"CEP,omitempty"`
	UF      string `json:"UF,omitempty"`
	Fone    string `json:"fone,omitempty"`
	Email   string `json:"email,omitempty"`
}

// InfModal espelha MdfeSefazInfModal.
type InfModal struct {
	VersaoModal string `json:"versaoModal"`
	Rodo        *Rodo  `json:"rodo,omitempty"`
}

// Rodo espelha MdfeSefazRodo.
type Rodo struct {
	InfANTT     *InfANTT      `json:"infANTT,omitempty"`
	VeicTracao  VeicTracao    `json:"veicTracao"`
	VeicReboque []VeicReboque `json:"veicReboque,omitempty"`
	CodAgPorto  string        `json:"codAgPorto,omitempty"`
	LacRodo     []LacRodo     `json:"lacRodo,omitempty"`
}

// InfANTT espelha MdfeSefazInfANTT.
type InfANTT struct {
	RNTRC          string           `json:"RNTRC,omitempty"`
	InfCIOT        []InfCIOT        `json:"infCIOT,omitempty"`
	ValePed        *ValePed         `json:"valePed,omitempty"`
	InfContratante []InfContratante `json:"infContratante,omitempty"`
	InfPag         []InfPag         `json:"infPag,omitempty"`
}

// InfCIOT espelha MdfeSefazInfCIOT.
type InfCIOT struct {
	CIOT string `json:"CIOT,omitempty"`
	CPF  string `json:"CPF,omitempty"`
	CNPJ string `json:"CNPJ,omitempty"`
}

// ValePed espelha MdfeSefazValePed.
type ValePed struct {
	Disp          []Disp `json:"disp"`
	CategCombVeic string `json:"categCombVeic,omitempty" enum:"CategoriaCombinacaoVeicular"`
}

// Disp espelha MdfeSefazDisp.
type Disp struct {
	CNPJForn  string  `json:"CNPJForn"`
	CNPJPg    string  `json:"CNPJPg,omitempty"`
	CPFPg     string  `json:"CPFPg,omitempty"`
	NCompra   string  `json:"nCompra,omitempty"`
	VValePed  float64 `json:"vValePed"`
	TpValePed string  `json:"tpValePed,omitempty" enum:"TipoValePedagio"`
}

// InfContratante espelha MdfeSefazInfContratante.
type InfContratante struct {
	XNome         string       `json:"xNome,omitempty"`
	CPF           string       `json:"CPF,omitempty"`
	CNPJ          string       `json:"CNPJ,omitempty"`
	IdEstrangeiro string       `json:"idEstrangeiro,omitempty"`
	InfContrato   *InfContrato `json:"infContrato,omitempty"`
}

// InfContrato espelha MdfeSefazInfContrato.
type InfContrato struct {
	NroContrato     string  `json:"NroContrato"`
	VContratoGlobal float64 `json:"vContratoGlobal"`
}

// InfPag espelha MdfeSefazInfPag.
type InfPag struct {
	XNome             string     `json:"xNome,omitempty"`
	CPF               string     `json:"CPF,omitempty"`
	CNPJ              string     `json:"CNPJ,omitempty"`
	IdEstrangeiro     string     `json:"idEstrangeiro,omitempty"`
	Comp              []Comp     `json:"Comp"`
	VContrato         float64    `json:"vContrato"`
	IndAltoDesemp     int        `json:"indAltoDesemp,omitempty" enum:"Sim"`
	IndPag            int        `json:"indPag" enum:"IndicadorPagamento"`
	VAdiant           float64    `json:"vAdiant,omitempty"`
	IndAntecipaAdiant int        `json:"indAntecipaAdiant,omitempty" enum:"Sim"`
	InfPrazo          []InfPrazo `json:"infPrazo,omitempty"`
	TpAntecip         int        `json:"tpAntecip,omitempty" enum:"TipoAntecipacaoParcela"`
	InfBanc           InfBanc    `json:"infBanc"`
}

// Comp espelha MdfeSefazComp.
type Comp struct {
	TpComp string  `json:"tpComp" enum:"TipoComponentePagamento"`
	VComp  float64 `json:"vComp"`
	XComp  string  `json:"xComp,omitempty"`
}

// InfPrazo espelha MdfeSefazInfPrazo.
type InfPrazo struct {
	NParcela int     `json:"nParcela"`
	DVenc    string  `json:"dVenc" fmt:"data"`
	VParcela float64 `json:"vParcela"`
}

// InfBanc espelha MdfeSefazInfBanc.
type InfBanc struct {
	CodBanco   string `json:"codBanco,omitempty"`
	CodAgencia string `json:"codAgencia,omitempty"`
	CNPJIPEF   string `json:"CNPJIPEF,omitempty"`
	PIX        string `json:"PIX,omitempty"`
}

// VeicTracao espelha MdfeSefazVeicTracao.
type VeicTracao struct {
	CInt     string     `json:"cInt,omitempty"`
	Placa    string     `json:"placa"`
	RENAVAM  string     `json:"RENAVAM,omitempty"`
	Tara     int        `json:"tara"`
	CapKG    int        `json:"capKG,omitempty"`
	CapM3    int        `json:"capM3,omitempty"`
	Prop     *Prop      `json:"prop,omitempty"`
	Condutor []Condutor `json:"condutor"`
	TpRod    string     `json:"tpRod" enum:"TipoRodado"`
	TpCar    string     `json:"tpCar" enum:"TipoCarroceria"`
	UF       string     `json:"UF,omitempty"`
}

// Prop é o proprietário do veículo, e vale tanto para a tração quanto para o
// reboque. Havia um tipo por posição, com os mesmos campos.
type Prop struct {
	CPF    string `json:"CPF,omitempty"`
	CNPJ   string `json:"CNPJ,omitempty"`
	RNTRC  string `json:"RNTRC"`
	XNome  string `json:"xNome"`
	IE     string `json:"IE,omitempty"`
	UF     string `json:"UF,omitempty"`
	TpProp int    `json:"tpProp" enum:"TipoPropriedadeVeiculo"`
}

// Condutor espelha MdfeSefazCondutor.
type Condutor struct {
	XNome string `json:"xNome"`
	CPF   string `json:"CPF"`
}

// VeicReboque espelha MdfeSefazVeicReboque.
type VeicReboque struct {
	CInt    string `json:"cInt,omitempty"`
	Placa   string `json:"placa"`
	RENAVAM string `json:"RENAVAM,omitempty"`
	Tara    int    `json:"tara"`
	CapKG   int    `json:"capKG"`
	CapM3   int    `json:"capM3,omitempty"`
	Prop    *Prop  `json:"prop,omitempty"`
	TpCar   string `json:"tpCar" enum:"TipoCarroceria"`
	UF      string `json:"UF,omitempty"`
}

// LacRodo espelha MdfeSefazLacRodo.
type LacRodo struct {
	NLacre string `json:"nLacre"`
}

// InfDoc espelha MdfeSefazInfDoc.
type InfDoc struct {
	InfMunDescarga []InfMunDescarga `json:"infMunDescarga"`
}

// InfMunDescarga espelha MdfeSefazInfMunDescarga.
type InfMunDescarga struct {
	CMunDescarga  string          `json:"cMunDescarga"`
	XMunDescarga  string          `json:"xMunDescarga"`
	InfCTe        []InfCTe        `json:"infCTe,omitempty"`
	InfNFe        []InfNFe        `json:"infNFe,omitempty"`
	InfMDFeTransp []InfMDFeTransp `json:"infMDFeTransp,omitempty"`
}

// InfCTe espelha MdfeSefazInfCTe.
type InfCTe struct {
	ChCTe               string               `json:"chCTe"`
	SegCodBarra         string               `json:"SegCodBarra,omitempty"`
	IndReentrega        int                  `json:"indReentrega,omitempty" enum:"Sim"`
	InfUnidTransp       []UnidadeTransp      `json:"infUnidTransp,omitempty"`
	Peri                []Peri               `json:"peri,omitempty"`
	InfEntregaParcial   *InfEntregaParcial   `json:"infEntregaParcial,omitempty"`
	IndPrestacaoParcial int                  `json:"indPrestacaoParcial,omitempty" enum:"Sim"`
	InfNFePrestParcial  []InfNFePrestParcial `json:"infNFePrestParcial,omitempty"`
}

// UnidadeTransp espelha MdfeSefazUnidadeTransp.
type UnidadeTransp struct {
	TpUnidTransp  int             `json:"tpUnidTransp" enum:"TipoUnidadeTransporte"`
	IdUnidTransp  string          `json:"idUnidTransp"`
	LacUnidTransp []LacUnidTransp `json:"lacUnidTransp,omitempty"`
	InfUnidCarga  []UnidCarga     `json:"infUnidCarga,omitempty"`
	QtdRat        float64         `json:"qtdRat,omitempty"`
}

// LacUnidTransp espelha MdfeSefazLacUnidTransp.
type LacUnidTransp struct {
	NLacre string `json:"nLacre"`
}

// UnidCarga espelha MdfeSefazUnidCarga.
type UnidCarga struct {
	TpUnidCarga  int            `json:"tpUnidCarga" enum:"TipoUnidadeCarga"`
	IdUnidCarga  string         `json:"idUnidCarga"`
	LacUnidCarga []LacUnidCarga `json:"lacUnidCarga,omitempty"`
	QtdRat       float64        `json:"qtdRat,omitempty"`
}

// LacUnidCarga espelha MdfeSefazLacUnidCarga.
type LacUnidCarga struct {
	NLacre string `json:"nLacre"`
}

// Peri é o produto perigoso transportado, e vale para os três documentos da
// descarga (NF-e, CT-e e MDF-e). Havia um tipo por documento, com os mesmos
// campos: acertar um deles e esquecer os outros dois era só questão de tempo.
type Peri struct {
	NONU      string `json:"nONU"`
	XNomeAE   string `json:"xNomeAE,omitempty"`
	XClaRisco string `json:"xClaRisco,omitempty"`
	GrEmb     string `json:"grEmb,omitempty"`
	QTotProd  string `json:"qTotProd"`
	QVolTipo  string `json:"qVolTipo,omitempty"`
}

// InfEntregaParcial espelha MdfeSefazInfEntregaParcial.
type InfEntregaParcial struct {
	QtdTotal   float64 `json:"qtdTotal"`
	QtdParcial float64 `json:"qtdParcial"`
}

// InfNFePrestParcial espelha MdfeSefazInfNFePrestParcial.
type InfNFePrestParcial struct {
	ChNFe string `json:"chNFe"`
}

// InfNFe espelha MdfeSefazInfNFe.
type InfNFe struct {
	ChNFe         string          `json:"chNFe"`
	SegCodBarra   string          `json:"SegCodBarra,omitempty"`
	IndReentrega  int             `json:"indReentrega,omitempty" enum:"Sim"`
	InfUnidTransp []UnidadeTransp `json:"infUnidTransp,omitempty"`
	Peri          []Peri          `json:"peri,omitempty"`
}

// InfMDFeTransp espelha MdfeSefazInfMDFeTransp.
type InfMDFeTransp struct {
	ChMDFe        string          `json:"chMDFe"`
	IndReentrega  int             `json:"indReentrega,omitempty" enum:"Sim"`
	InfUnidTransp []UnidadeTransp `json:"infUnidTransp,omitempty"`
	Peri          []Peri          `json:"peri,omitempty"`
}

// Seg espelha MdfeSefazSeg.
type Seg struct {
	InfResp InfResp  `json:"infResp"`
	InfSeg  *InfSeg  `json:"infSeg,omitempty"`
	NApol   string   `json:"nApol,omitempty"`
	NAver   []string `json:"nAver,omitempty"`
}

// InfResp espelha MdfeSefazInfResp.
type InfResp struct {
	RespSeg int    `json:"respSeg" enum:"ResponsavelSeguro"`
	CNPJ    string `json:"CNPJ,omitempty"`
	CPF     string `json:"CPF,omitempty"`
}

// InfSeg espelha MdfeSefazInfSeg.
type InfSeg struct {
	XSeg string `json:"xSeg"`
	CNPJ string `json:"CNPJ"`
}

// ProdPred espelha MdfeSefazProdPred.
type ProdPred struct {
	TpCarga    string      `json:"tpCarga" enum:"TipoCarga"`
	XProd      string      `json:"xProd"`
	CEAN       string      `json:"cEAN,omitempty"`
	NCM        string      `json:"NCM,omitempty"`
	InfLotacao *InfLotacao `json:"infLotacao,omitempty"`
}

// InfLotacao espelha MdfeSefazInfLotacao.
type InfLotacao struct {
	InfLocalCarrega    InfLocalCarrega    `json:"infLocalCarrega"`
	InfLocalDescarrega InfLocalDescarrega `json:"infLocalDescarrega"`
}

// InfLocalCarrega espelha MdfeSefazInfLocalCarrega.
type InfLocalCarrega struct {
	CEP       string `json:"CEP,omitempty"`
	Latitude  string `json:"latitude,omitempty"`
	Longitude string `json:"longitude,omitempty"`
}

// InfLocalDescarrega espelha MdfeSefazInfLocalDescarrega.
type InfLocalDescarrega struct {
	CEP       string `json:"CEP,omitempty"`
	Latitude  string `json:"latitude,omitempty"`
	Longitude string `json:"longitude,omitempty"`
}

// Tot espelha MdfeSefazTot.
type Tot struct {
	QCTe   int     `json:"qCTe,omitempty"`
	QNFe   int     `json:"qNFe,omitempty"`
	QMDFe  int     `json:"qMDFe,omitempty"`
	VCarga float64 `json:"vCarga"`
	CUnid  string  `json:"cUnid" enum:"UnidadeMedidaPeso"`
	QCarga float64 `json:"qCarga"`
}

// Lacres espelha MdfeSefazLacres.
type Lacres struct {
	NLacre string `json:"nLacre"`
}

// AutXML espelha MdfeSefazAutXML.
type AutXML struct {
	CNPJ string `json:"CNPJ,omitempty"`
	CPF  string `json:"CPF,omitempty"`
}

// InfAdic espelha MdfeSefazInfAdic.
type InfAdic struct {
	InfAdFisco string `json:"infAdFisco,omitempty"`
	InfCpl     string `json:"infCpl,omitempty"`
}

// RespTec espelha MdfeSefazRespTec.
type RespTec struct {
	CNPJ     string `json:"CNPJ"`
	XContato string `json:"xContato"`
	Email    string `json:"email"`
	Fone     string `json:"fone"`
	IdCSRT   int    `json:"idCSRT,omitempty"`
	CSRT     string `json:"CSRT,omitempty"`
	HashCSRT string `json:"hashCSRT,omitempty"`
}

// String e LogValue redigem o tipo inteiro por causa do CSRT: ele é o código de
// segurança do responsável técnico, um segredo compartilhado com o fisco, e com
// ele se assina em nome de quem o registrou. Vai no payload, e log é a única
// forma realista de ele escapar deste processo.
//
// A serialização JSON continua carregando tudo: a redação é contra LOG, não
// contra transporte.
func (r RespTec) String() string { return "RespTec{redigido}" }

func (r RespTec) LogValue() slog.Value { return slog.StringValue("RespTec{redigido}") }

// InfSolicNFF espelha MdfeSefazInfSolicNFF.
type InfSolicNFF struct {
	XSolic string `json:"xSolic"`
}

// InfMDFeSupl espelha MdfeSefazInfMDFeSupl.
type InfMDFeSupl struct {
	QrCodMDFe string `json:"qrCodMDFe,omitempty"`
}
