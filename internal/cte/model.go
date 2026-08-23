// Package cte traduz o contrato JSON do CT-e (modelo 57) — espelhando a
// ACBr.API (CtePedidoEmissao = layout SEFAZ COMPLETO) — para o INI consumido
// pela ACBrLibCTe (CTE_CarregarINI), e interpreta as respostas.
//
// Os DTOs abaixo cobrem 100% das tags do contrato (gerados do contrato oficial).
// A tradução ToINI (ini.go) cobre o caminho rodoviário + grupos comuns; grupos
// exóticos (outros modais, Reforma Tributária IBSCBS) existem no payload e têm
// o ToINI marcado como pendente de verificação contra a lib.
package cte

// PedidoEmissao espelha CtePedidoEmissao.
type PedidoEmissao struct {
	InfCte     InfCte      `json:"infCte"`
	InfCTeSupl *InfCTeSupl `json:"infCTeSupl,omitempty"`
	Ambiente   string      `json:"ambiente"`
	Referencia string      `json:"referencia,omitempty"`
}

// InfCte espelha CteSefazInfCte.
type InfCte struct {
	Versao      string       `json:"versao"`
	Id          string       `json:"Id,omitempty"`
	Ide         Ide          `json:"ide"`
	Compl       *Compl       `json:"compl,omitempty"`
	Emit        Emit         `json:"emit"`
	Rem         *Rem         `json:"rem,omitempty"`
	Exped       *Exped       `json:"exped,omitempty"`
	Receb       *Receb       `json:"receb,omitempty"`
	Dest        *Dest        `json:"dest,omitempty"`
	VPrest      VPrest       `json:"vPrest"`
	Imp         Imp          `json:"imp"`
	InfCTeNorm  *InfCTeNorm  `json:"infCTeNorm,omitempty"`
	InfCteComp  []InfCteComp `json:"infCteComp,omitempty"`
	AutXML      []AutXML     `json:"autXML,omitempty"`
	InfRespTec  *RespTec     `json:"infRespTec,omitempty"`
	InfSolicNFF *InfSolicNFF `json:"infSolicNFF,omitempty"`
}

// Ide espelha CteSefazIde.
type Ide struct {
	CUF            int                `json:"cUF"`
	CCT            string             `json:"cCT,omitempty"`
	CFOP           string             `json:"CFOP"`
	NatOp          string             `json:"natOp"`
	Mod            int                `json:"mod,omitempty"`
	Serie          int                `json:"serie"`
	NCT            int                `json:"nCT"`
	DhEmi          string             `json:"dhEmi"`
	TpImp          int                `json:"tpImp"`
	TpEmis         int                `json:"tpEmis"`
	CDV            int                `json:"cDV,omitempty"`
	TpAmb          int                `json:"tpAmb,omitempty"`
	TpCTe          int                `json:"tpCTe"`
	ProcEmi        int                `json:"procEmi"`
	VerProc        string             `json:"verProc"`
	IndGlobalizado int                `json:"indGlobalizado,omitempty"`
	CMunEnv        string             `json:"cMunEnv"`
	XMunEnv        string             `json:"xMunEnv"`
	UFEnv          string             `json:"UFEnv"`
	Modal          string             `json:"modal"`
	TpServ         int                `json:"tpServ"`
	CMunIni        string             `json:"cMunIni"`
	XMunIni        string             `json:"xMunIni"`
	UFIni          string             `json:"UFIni"`
	CMunFim        string             `json:"cMunFim"`
	XMunFim        string             `json:"xMunFim"`
	UFFim          string             `json:"UFFim"`
	Retira         int                `json:"retira"`
	XDetRetira     string             `json:"xDetRetira,omitempty"`
	IndIEToma      int                `json:"indIEToma"`
	Toma3          *Toma3             `json:"toma3,omitempty"`
	Toma4          *Toma4             `json:"toma4,omitempty"`
	DhCont         string             `json:"dhCont,omitempty"`
	XJust          string             `json:"xJust,omitempty"`
	GCompraGov     *CompraGovReduzido `json:"gCompraGov,omitempty"`
}

// Toma3 espelha CteSefazToma3.
type Toma3 struct {
	Toma int `json:"toma"`
}

// Toma4 espelha CteSefazToma4.
type Toma4 struct {
	Toma      int      `json:"toma"`
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	XNome     string   `json:"xNome"`
	XFant     string   `json:"xFant,omitempty"`
	Fone      string   `json:"fone,omitempty"`
	EnderToma Endereco `json:"enderToma"`
	Email     string   `json:"email,omitempty"`
}

// Endereco espelha CteSefazEndereco.
type Endereco struct {
	XLgr    string `json:"xLgr"`
	Nro     string `json:"nro"`
	XCpl    string `json:"xCpl,omitempty"`
	XBairro string `json:"xBairro"`
	CMun    string `json:"cMun"`
	XMun    string `json:"xMun"`
	CEP     string `json:"CEP,omitempty"`
	UF      string `json:"UF"`
	CPais   string `json:"cPais,omitempty"`
	XPais   string `json:"xPais,omitempty"`
}

// CompraGovReduzido espelha CteSefazCompraGovReduzido.
type CompraGovReduzido struct {
	TpEnteGov int     `json:"tpEnteGov"`
	PRedutor  float64 `json:"pRedutor"`
}

// Compl espelha CteSefazCompl.
type Compl struct {
	XCaracAd  string     `json:"xCaracAd,omitempty"`
	XCaracSer string     `json:"xCaracSer,omitempty"`
	XEmi      string     `json:"xEmi,omitempty"`
	Fluxo     *Fluxo     `json:"fluxo,omitempty"`
	Entrega   *Entrega   `json:"Entrega,omitempty"`
	OrigCalc  string     `json:"origCalc,omitempty"`
	DestCalc  string     `json:"destCalc,omitempty"`
	XObs      string     `json:"xObs,omitempty"`
	ObsCont   []ObsCont  `json:"ObsCont,omitempty"`
	ObsFisco  []ObsFisco `json:"ObsFisco,omitempty"`
}

// Fluxo espelha CteSefazFluxo.
type Fluxo struct {
	XOrig string `json:"xOrig,omitempty"`
	Pass  []Pass `json:"pass,omitempty"`
	XDest string `json:"xDest,omitempty"`
	XRota string `json:"xRota,omitempty"`
}

// Pass espelha CteSefazPass.
type Pass struct {
	XPass string `json:"xPass,omitempty"`
}

// Entrega espelha CteSefazEntrega.
type Entrega struct {
	SemData   *SemData   `json:"semData,omitempty"`
	ComData   *ComData   `json:"comData,omitempty"`
	NoPeriodo *NoPeriodo `json:"noPeriodo,omitempty"`
	SemHora   *SemHora   `json:"semHora,omitempty"`
	ComHora   *ComHora   `json:"comHora,omitempty"`
	NoInter   *NoInter   `json:"noInter,omitempty"`
}

// SemData espelha CteSefazSemData.
type SemData struct {
	TpPer int `json:"tpPer"`
}

// ComData espelha CteSefazComData.
type ComData struct {
	TpPer int    `json:"tpPer"`
	DProg string `json:"dProg"`
}

// NoPeriodo espelha CteSefazNoPeriodo.
type NoPeriodo struct {
	TpPer int    `json:"tpPer"`
	DIni  string `json:"dIni"`
	DFim  string `json:"dFim"`
}

// SemHora espelha CteSefazSemHora.
type SemHora struct {
	TpHor int `json:"tpHor"`
}

// ComHora espelha CteSefazComHora.
type ComHora struct {
	TpHor int    `json:"tpHor"`
	HProg string `json:"hProg"`
}

// NoInter espelha CteSefazNoInter.
type NoInter struct {
	TpHor int    `json:"tpHor"`
	HIni  string `json:"hIni"`
	HFim  string `json:"hFim"`
}

// ObsCont espelha CteSefazObsCont.
type ObsCont struct {
	XCampo string `json:"xCampo"`
	XTexto string `json:"xTexto"`
}

// ObsFisco espelha CteSefazObsFisco.
type ObsFisco struct {
	XCampo string `json:"xCampo"`
	XTexto string `json:"xTexto"`
}

// Emit espelha CteSefazEmit.
type Emit struct {
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	IEST      string   `json:"IEST,omitempty"`
	XNome     string   `json:"xNome,omitempty"`
	XFant     string   `json:"xFant,omitempty"`
	EnderEmit *EndeEmi `json:"enderEmit,omitempty"`
	CRT       int      `json:"CRT,omitempty"`
}

// EndeEmi espelha CteSefazEndeEmi.
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
}

// Rem espelha CteSefazRem.
type Rem struct {
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	XNome     string   `json:"xNome"`
	XFant     string   `json:"xFant,omitempty"`
	Fone      string   `json:"fone,omitempty"`
	EnderReme Endereco `json:"enderReme"`
	Email     string   `json:"email,omitempty"`
}

// Exped espelha CteSefazExped.
type Exped struct {
	CNPJ       string   `json:"CNPJ,omitempty"`
	CPF        string   `json:"CPF,omitempty"`
	IE         string   `json:"IE,omitempty"`
	XNome      string   `json:"xNome"`
	Fone       string   `json:"fone,omitempty"`
	EnderExped Endereco `json:"enderExped"`
	Email      string   `json:"email,omitempty"`
}

// Receb espelha CteSefazReceb.
type Receb struct {
	CNPJ       string   `json:"CNPJ,omitempty"`
	CPF        string   `json:"CPF,omitempty"`
	IE         string   `json:"IE,omitempty"`
	XNome      string   `json:"xNome"`
	Fone       string   `json:"fone,omitempty"`
	EnderReceb Endereco `json:"enderReceb"`
	Email      string   `json:"email,omitempty"`
}

// Dest espelha CteSefazDest.
type Dest struct {
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	XNome     string   `json:"xNome"`
	Fone      string   `json:"fone,omitempty"`
	ISUF      string   `json:"ISUF,omitempty"`
	EnderDest Endereco `json:"enderDest"`
	Email     string   `json:"email,omitempty"`
}

// VPrest espelha CteSefazVPrest.
type VPrest struct {
	VTPrest float64 `json:"vTPrest"`
	VRec    float64 `json:"vRec"`
	Comp    []Comp  `json:"Comp,omitempty"`
}

// Comp espelha CteSefazComp.
type Comp struct {
	XNome string  `json:"xNome"`
	VComp float64 `json:"vComp"`
}

// Imp espelha CteSefazInfCte_Imp.
type Imp struct {
	ICMS       ICMS       `json:"ICMS"`
	VTotTrib   float64    `json:"vTotTrib,omitempty"`
	InfAdFisco string     `json:"infAdFisco,omitempty"`
	ICMSUFFim  *ICMSUFFim `json:"ICMSUFFim,omitempty"`
	IBSCBS     *TribCTe   `json:"IBSCBS,omitempty"`
	VTotDFe    float64    `json:"vTotDFe,omitempty"`
}

// ICMS espelha CteSefazImp.
type ICMS struct {
	ICMS00      *ICMS00      `json:"ICMS00,omitempty"`
	ICMS20      *ICMS20      `json:"ICMS20,omitempty"`
	ICMS45      *ICMS45      `json:"ICMS45,omitempty"`
	ICMS60      *ICMS60      `json:"ICMS60,omitempty"`
	ICMS90      *ICMS90      `json:"ICMS90,omitempty"`
	ICMSOutraUF *ICMSOutraUF `json:"ICMSOutraUF,omitempty"`
	ICMSSN      *ICMSSN      `json:"ICMSSN,omitempty"`
}

// ICMS00 espelha CteSefazICMS00.
type ICMS00 struct {
	CST   string  `json:"CST"`
	VBC   float64 `json:"vBC"`
	PICMS float64 `json:"pICMS"`
	VICMS float64 `json:"vICMS"`
}

// ICMS20 espelha CteSefazICMS20.
type ICMS20 struct {
	CST        string  `json:"CST"`
	PRedBC     float64 `json:"pRedBC"`
	VBC        float64 `json:"vBC"`
	PICMS      float64 `json:"pICMS"`
	VICMS      float64 `json:"vICMS"`
	VICMSDeson float64 `json:"vICMSDeson,omitempty"`
	CBenef     string  `json:"cBenef,omitempty"`
}

// ICMS45 espelha CteSefazICMS45.
type ICMS45 struct {
	CST        string  `json:"CST"`
	VICMSDeson float64 `json:"vICMSDeson,omitempty"`
	CBenef     string  `json:"cBenef,omitempty"`
}

// ICMS60 espelha CteSefazICMS60.
type ICMS60 struct {
	CST        string  `json:"CST"`
	VBCSTRet   float64 `json:"vBCSTRet"`
	VICMSSTRet float64 `json:"vICMSSTRet"`
	PICMSSTRet float64 `json:"pICMSSTRet"`
	VCred      float64 `json:"vCred,omitempty"`
	VICMSDeson float64 `json:"vICMSDeson,omitempty"`
	CBenef     string  `json:"cBenef,omitempty"`
}

// ICMS90 espelha CteSefazICMS90.
type ICMS90 struct {
	CST        string  `json:"CST"`
	PRedBC     float64 `json:"pRedBC,omitempty"`
	VBC        float64 `json:"vBC"`
	PICMS      float64 `json:"pICMS"`
	VICMS      float64 `json:"vICMS"`
	VCred      float64 `json:"vCred,omitempty"`
	VICMSDeson float64 `json:"vICMSDeson,omitempty"`
	CBenef     string  `json:"cBenef,omitempty"`
}

// ICMSOutraUF espelha CteSefazICMSOutraUF.
type ICMSOutraUF struct {
	CST           string  `json:"CST"`
	PRedBCOutraUF float64 `json:"pRedBCOutraUF,omitempty"`
	VBCOutraUF    float64 `json:"vBCOutraUF"`
	PICMSOutraUF  float64 `json:"pICMSOutraUF"`
	VICMSOutraUF  float64 `json:"vICMSOutraUF"`
	VICMSDeson    float64 `json:"vICMSDeson,omitempty"`
	CBenef        string  `json:"cBenef,omitempty"`
}

// ICMSSN espelha CteSefazICMSSN.
type ICMSSN struct {
	CST   string `json:"CST"`
	IndSN int    `json:"indSN"`
}

// ICMSUFFim espelha CteSefazICMSUFFim.
type ICMSUFFim struct {
	VBCUFFim   float64 `json:"vBCUFFim"`
	PFCPUFFim  float64 `json:"pFCPUFFim"`
	PICMSUFFim float64 `json:"pICMSUFFim"`
	PICMSInter float64 `json:"pICMSInter"`
	VFCPUFFim  float64 `json:"vFCPUFFim"`
	VICMSUFFim float64 `json:"vICMSUFFim"`
	VICMSUFIni float64 `json:"vICMSUFIni"`
}

// TribCTe espelha CteSefazTribCTe.
type TribCTe struct {
	CST          string       `json:"CST"`
	CClassTrib   string       `json:"cClassTrib,omitempty"`
	IndDoacao    int          `json:"indDoacao,omitempty"`
	GIBSCBS      *CIBS        `json:"gIBSCBS,omitempty"`
	GEstornoCred *EstornoCred `json:"gEstornoCred,omitempty"`
}

// CIBS espelha CteSefazCIBS.
type CIBS struct {
	VBC            float64        `json:"vBC"`
	GIBSUF         GIBSUF         `json:"gIBSUF"`
	GIBSMun        GIBSMun        `json:"gIBSMun"`
	VIBS           float64        `json:"vIBS"`
	GCBS           GCBS           `json:"gCBS"`
	GTribRegular   *TribRegular   `json:"gTribRegular,omitempty"`
	GTribCompraGov *TribCompraGov `json:"gTribCompraGov,omitempty"`
}

// GIBSUF espelha CteSefazGIBSUF.
type GIBSUF struct {
	PIBSUF   float64  `json:"pIBSUF"`
	GDif     *Dif     `json:"gDif,omitempty"`
	GDevTrib *DevTrib `json:"gDevTrib,omitempty"`
	GRed     *Red     `json:"gRed,omitempty"`
	VIBSUF   float64  `json:"vIBSUF"`
}

// Dif espelha CteSefazDif.
type Dif struct {
	PDif float64 `json:"pDif"`
	VDif float64 `json:"vDif"`
}

// DevTrib espelha CteSefazDevTrib.
type DevTrib struct {
	VDevTrib float64 `json:"vDevTrib"`
}

// Red espelha CteSefazRed.
type Red struct {
	PRedAliq  float64 `json:"pRedAliq"`
	PAliqEfet float64 `json:"pAliqEfet"`
}

// GIBSMun espelha CteSefazGIBSMun.
type GIBSMun struct {
	PIBSMun  float64  `json:"pIBSMun"`
	GDif     *Dif     `json:"gDif,omitempty"`
	GDevTrib *DevTrib `json:"gDevTrib,omitempty"`
	GRed     *Red     `json:"gRed,omitempty"`
	VIBSMun  float64  `json:"vIBSMun"`
}

// GCBS espelha CteSefazGCBS.
type GCBS struct {
	PCBS     float64  `json:"pCBS"`
	GDif     *Dif     `json:"gDif,omitempty"`
	GDevTrib *DevTrib `json:"gDevTrib,omitempty"`
	GRed     *Red     `json:"gRed,omitempty"`
	VCBS     float64  `json:"vCBS"`
}

// TribRegular espelha CteSefazTribRegular.
type TribRegular struct {
	CSTReg             string  `json:"CSTReg"`
	CClassTribReg      string  `json:"cClassTribReg"`
	PAliqEfetRegIBSUF  float64 `json:"pAliqEfetRegIBSUF"`
	VTribRegIBSUF      float64 `json:"vTribRegIBSUF"`
	PAliqEfetRegIBSMun float64 `json:"pAliqEfetRegIBSMun"`
	VTribRegIBSMun     float64 `json:"vTribRegIBSMun"`
	PAliqEfetRegCBS    float64 `json:"pAliqEfetRegCBS"`
	VTribRegCBS        float64 `json:"vTribRegCBS"`
}

// TribCompraGov espelha CteSefazTribCompraGov.
type TribCompraGov struct {
	PAliqIBSUF  float64 `json:"pAliqIBSUF,omitempty"`
	VTribIBSUF  float64 `json:"vTribIBSUF"`
	PAliqIBSMun float64 `json:"pAliqIBSMun,omitempty"`
	VTribIBSMun float64 `json:"vTribIBSMun"`
	PAliqCBS    float64 `json:"pAliqCBS,omitempty"`
	VTribCBS    float64 `json:"vTribCBS"`
}

// EstornoCred espelha CteSefazEstornoCred.
type EstornoCred struct {
	VIBSEstCred float64 `json:"vIBSEstCred"`
	VCBSEstCred float64 `json:"vCBSEstCred"`
}

// InfCTeNorm espelha CteSefazInfCTeNorm.
type InfCTeNorm struct {
	InfCarga       InfCarga        `json:"infCarga"`
	InfDoc         *InfDoc         `json:"infDoc,omitempty"`
	DocAnt         *DocAnt         `json:"docAnt,omitempty"`
	InfModal       InfModal        `json:"infModal"`
	VeicNovos      []VeicNovos     `json:"veicNovos,omitempty"`
	Cobr           *Cobr           `json:"cobr,omitempty"`
	InfCteSub      *InfCteSub      `json:"infCteSub,omitempty"`
	InfGlobalizado *InfGlobalizado `json:"infGlobalizado,omitempty"`
	InfServVinc    *InfServVinc    `json:"infServVinc,omitempty"`
}

// InfCarga espelha CteSefazInfCarga.
type InfCarga struct {
	VCarga      float64 `json:"vCarga,omitempty"`
	ProPred     string  `json:"proPred"`
	XOutCat     string  `json:"xOutCat,omitempty"`
	InfQ        []InfQ  `json:"infQ"`
	VCargaAverb float64 `json:"vCargaAverb,omitempty"`
}

// InfQ espelha CteSefazInfQ.
type InfQ struct {
	CUnid  string  `json:"cUnid"`
	TpMed  string  `json:"tpMed"`
	QCarga float64 `json:"qCarga"`
}

// InfDoc espelha CteSefazInfDoc.
type InfDoc struct {
	InfNF     []InfNF     `json:"infNF,omitempty"`
	InfNFe    []InfNFe    `json:"infNFe,omitempty"`
	InfOutros []InfOutros `json:"infOutros,omitempty"`
	InfDCe    []InfDCe    `json:"infDCe,omitempty"`
}

// InfNF espelha CteSefazInfNF.
type InfNF struct {
	NRoma         string          `json:"nRoma,omitempty"`
	NPed          string          `json:"nPed,omitempty"`
	Mod           string          `json:"mod"`
	Serie         string          `json:"serie"`
	NDoc          string          `json:"nDoc"`
	DEmi          string          `json:"dEmi"`
	VBC           float64         `json:"vBC"`
	VICMS         float64         `json:"vICMS"`
	VBCST         float64         `json:"vBCST"`
	VST           float64         `json:"vST"`
	VProd         float64         `json:"vProd"`
	VNF           float64         `json:"vNF"`
	NCFOP         string          `json:"nCFOP"`
	NPeso         float64         `json:"nPeso,omitempty"`
	PIN           string          `json:"PIN,omitempty"`
	DPrev         string          `json:"dPrev,omitempty"`
	InfUnidCarga  []UnidCarga     `json:"infUnidCarga,omitempty"`
	InfUnidTransp []UnidadeTransp `json:"infUnidTransp,omitempty"`
}

// UnidCarga espelha CteSefazUnidCarga.
type UnidCarga struct {
	TpUnidCarga  int            `json:"tpUnidCarga"`
	IdUnidCarga  string         `json:"idUnidCarga"`
	LacUnidCarga []LacUnidCarga `json:"lacUnidCarga,omitempty"`
	QtdRat       float64        `json:"qtdRat,omitempty"`
}

// LacUnidCarga espelha CteSefazLacUnidCarga.
type LacUnidCarga struct {
	NLacre string `json:"nLacre"`
}

// UnidadeTransp espelha CteSefazUnidadeTransp.
type UnidadeTransp struct {
	TpUnidTransp  int             `json:"tpUnidTransp"`
	IdUnidTransp  string          `json:"idUnidTransp"`
	LacUnidTransp []LacUnidTransp `json:"lacUnidTransp,omitempty"`
	InfUnidCarga  []UnidCarga     `json:"infUnidCarga,omitempty"`
	QtdRat        float64         `json:"qtdRat,omitempty"`
}

// LacUnidTransp espelha CteSefazLacUnidTransp.
type LacUnidTransp struct {
	NLacre string `json:"nLacre"`
}

// InfNFe espelha CteSefazInfNFe.
type InfNFe struct {
	Chave         string          `json:"chave"`
	PIN           string          `json:"PIN,omitempty"`
	DPrev         string          `json:"dPrev,omitempty"`
	InfUnidCarga  []UnidCarga     `json:"infUnidCarga,omitempty"`
	InfUnidTransp []UnidadeTransp `json:"infUnidTransp,omitempty"`
}

// InfOutros espelha CteSefazInfOutros.
type InfOutros struct {
	TpDoc         string          `json:"tpDoc"`
	DescOutros    string          `json:"descOutros,omitempty"`
	NDoc          string          `json:"nDoc,omitempty"`
	DEmi          string          `json:"dEmi,omitempty"`
	VDocFisc      float64         `json:"vDocFisc,omitempty"`
	DPrev         string          `json:"dPrev,omitempty"`
	InfUnidCarga  []UnidCarga     `json:"infUnidCarga,omitempty"`
	InfUnidTransp []UnidadeTransp `json:"infUnidTransp,omitempty"`
}

// InfDCe espelha CteSefazInfDCe.
type InfDCe struct {
	Chave string `json:"chave"`
}

// DocAnt espelha CteSefazDocAnt.
type DocAnt struct {
	EmiDocAnt []EmiDocAnt `json:"emiDocAnt"`
}

// EmiDocAnt espelha CteSefazEmiDocAnt.
type EmiDocAnt struct {
	CNPJ     string     `json:"CNPJ,omitempty"`
	CPF      string     `json:"CPF,omitempty"`
	IE       string     `json:"IE,omitempty"`
	UF       string     `json:"UF,omitempty"`
	XNome    string     `json:"xNome"`
	IdDocAnt []IdDocAnt `json:"idDocAnt"`
}

// IdDocAnt espelha CteSefazIdDocAnt.
type IdDocAnt struct {
	IdDocAntPap []IdDocAntPap `json:"idDocAntPap,omitempty"`
	IdDocAntEle []IdDocAntEle `json:"idDocAntEle,omitempty"`
}

// IdDocAntPap espelha CteSefazIdDocAntPap.
type IdDocAntPap struct {
	TpDoc  string `json:"tpDoc"`
	Serie  string `json:"serie"`
	Subser string `json:"subser,omitempty"`
	NDoc   string `json:"nDoc"`
	DEmi   string `json:"dEmi"`
}

// IdDocAntEle espelha CteSefazIdDocAntEle.
type IdDocAntEle struct {
	ChCTe string `json:"chCTe"`
}

// InfModal espelha CteSefazInfModal.
type InfModal struct {
	VersaoModal string      `json:"versaoModal"`
	Rodo        *Rodo       `json:"rodo,omitempty"`
	Aereo       *Aereo      `json:"aereo,omitempty"`
	Ferrov      *Ferrov     `json:"ferrov,omitempty"`
	Aquav       *Aquav      `json:"aquav,omitempty"`
	Duto        *Duto       `json:"duto,omitempty"`
	Multimodal  *Multimodal `json:"multimodal,omitempty"`
}

// Rodo espelha CteSefazRodo.
type Rodo struct {
	RNTRC string `json:"RNTRC"`
	Occ   []Occ  `json:"occ,omitempty"`
}

// Occ espelha CteSefazOcc.
type Occ struct {
	Serie  string  `json:"serie,omitempty"`
	NOcc   int     `json:"nOcc"`
	DEmi   string  `json:"dEmi"`
	EmiOcc *EmiOcc `json:"emiOcc,omitempty"`
}

// EmiOcc espelha CteSefazEmiOcc.
type EmiOcc struct {
	CNPJ string `json:"CNPJ"`
	CInt string `json:"cInt,omitempty"`
	IE   string `json:"IE"`
	UF   string `json:"UF"`
	Fone string `json:"fone,omitempty"`
}

// Aereo espelha CteSefazAereo.
type Aereo struct {
	NMinu      int      `json:"nMinu,omitempty"`
	NOCA       string   `json:"nOCA,omitempty"`
	DPrevAereo string   `json:"dPrevAereo"`
	NatCarga   NatCarga `json:"natCarga"`
	Tarifa     Tarifa   `json:"tarifa"`
	Peri       []Peri   `json:"peri,omitempty"`
}

// NatCarga espelha CteSefazNatCarga.
type NatCarga struct {
	XDime    string   `json:"xDime,omitempty"`
	CInfManu []string `json:"cInfManu,omitempty"`
}

// Tarifa espelha CteSefazTarifa.
type Tarifa struct {
	CL   string  `json:"CL"`
	CTar string  `json:"cTar,omitempty"`
	VTar float64 `json:"vTar"`
}

// Peri espelha CteSefazPeri.
type Peri struct {
	NONU     string   `json:"nONU"`
	QTotEmb  string   `json:"qTotEmb"`
	InfTotAP InfTotAP `json:"infTotAP"`
}

// InfTotAP espelha CteSefazInfTotAP.
type InfTotAP struct {
	QTotProd float64 `json:"qTotProd"`
	UniAP    int     `json:"uniAP"`
}

// Ferrov espelha CteSefazFerrov.
type Ferrov struct {
	TpTraf  int      `json:"tpTraf"`
	TrafMut *TrafMut `json:"trafMut,omitempty"`
	Fluxo   string   `json:"fluxo"`
}

// TrafMut espelha CteSefazTrafMut.
type TrafMut struct {
	RespFat          int        `json:"respFat"`
	FerrEmi          int        `json:"ferrEmi"`
	VFrete           float64    `json:"vFrete"`
	ChCTeFerroOrigem string     `json:"chCTeFerroOrigem,omitempty"`
	FerroEnv         []FerroEnv `json:"ferroEnv,omitempty"`
}

// FerroEnv espelha CteSefazFerroEnv.
type FerroEnv struct {
	CNPJ       string   `json:"CNPJ"`
	CInt       string   `json:"cInt,omitempty"`
	IE         string   `json:"IE,omitempty"`
	XNome      string   `json:"xNome"`
	EnderFerro EnderFer `json:"enderFerro"`
}

// EnderFer espelha CteSefazEnderFer.
type EnderFer struct {
	XLgr    string `json:"xLgr"`
	Nro     string `json:"nro,omitempty"`
	XCpl    string `json:"xCpl,omitempty"`
	XBairro string `json:"xBairro,omitempty"`
	CMun    string `json:"cMun"`
	XMun    string `json:"xMun"`
	CEP     string `json:"CEP"`
	UF      string `json:"UF"`
}

// Aquav espelha CteSefazAquav.
type Aquav struct {
	VPrest  float64   `json:"vPrest"`
	VAFRMM  float64   `json:"vAFRMM"`
	XNavio  string    `json:"xNavio"`
	Balsa   []Balsa   `json:"balsa,omitempty"`
	NViag   string    `json:"nViag,omitempty"`
	Direc   string    `json:"direc"`
	Irin    string    `json:"irin"`
	DetCont []DetCont `json:"detCont,omitempty"`
	TpNav   int       `json:"tpNav,omitempty"`
}

// Balsa espelha CteSefazBalsa.
type Balsa struct {
	XBalsa string `json:"xBalsa"`
}

// DetCont espelha CteSefazDetCont.
type DetCont struct {
	NCont  string         `json:"nCont"`
	Lacre  []Lacre        `json:"lacre,omitempty"`
	InfDoc *DetContInfDoc `json:"infDoc,omitempty"`
}

// Lacre espelha CteSefazLacre.
type Lacre struct {
	NLacre string `json:"nLacre"`
}

// DetContInfDoc espelha CteSefazDetCont_InfDoc.
type DetContInfDoc struct {
	InfNF  []DetContInfNF  `json:"infNF,omitempty"`
	InfNFe []DetContInfNFe `json:"infNFe,omitempty"`
}

// DetContInfNF espelha CteSefazDetCont_InfDoc_InfNF.
type DetContInfNF struct {
	Serie   string  `json:"serie"`
	NDoc    string  `json:"nDoc"`
	UnidRat float64 `json:"unidRat,omitempty"`
}

// DetContInfNFe espelha CteSefazDetCont_InfDoc_InfNFe.
type DetContInfNFe struct {
	Chave   string  `json:"chave"`
	UnidRat float64 `json:"unidRat,omitempty"`
}

// Duto espelha CteSefazDuto.
type Duto struct {
	VTar            float64 `json:"vTar,omitempty"`
	DIni            string  `json:"dIni"`
	DFim            string  `json:"dFim"`
	ClassDuto       int     `json:"classDuto,omitempty"`
	TpContratacao   int     `json:"tpContratacao,omitempty"`
	CodPontoEntrada string  `json:"codPontoEntrada,omitempty"`
	CodPontoSaida   string  `json:"codPontoSaida,omitempty"`
	NContrato       string  `json:"nContrato,omitempty"`
}

// Multimodal espelha CteSefazMultimodal.
type Multimodal struct {
	COTM          string `json:"COTM"`
	IndNegociavel int    `json:"indNegociavel"`
	Seg           *Seg   `json:"seg,omitempty"`
}

// Seg espelha CteSefazSeg.
type Seg struct {
	InfSeg InfSeg `json:"infSeg"`
	NApol  string `json:"nApol"`
	NAver  string `json:"nAver"`
}

// InfSeg espelha CteSefazInfSeg.
type InfSeg struct {
	XSeg string `json:"xSeg"`
	CNPJ string `json:"CNPJ"`
}

// VeicNovos espelha CteSefazVeicNovos.
type VeicNovos struct {
	Chassi string  `json:"chassi"`
	CCor   string  `json:"cCor"`
	XCor   string  `json:"xCor"`
	CMod   string  `json:"cMod"`
	VUnit  float64 `json:"vUnit"`
	VFrete float64 `json:"vFrete"`
}

// Cobr espelha CteSefazCobr.
type Cobr struct {
	Fat *Fat  `json:"fat,omitempty"`
	Dup []Dup `json:"dup,omitempty"`
}

// Fat espelha CteSefazFat.
type Fat struct {
	NFat  string  `json:"nFat,omitempty"`
	VOrig float64 `json:"vOrig,omitempty"`
	VDesc float64 `json:"vDesc,omitempty"`
	VLiq  float64 `json:"vLiq,omitempty"`
}

// Dup espelha CteSefazDup.
type Dup struct {
	NDup  string  `json:"nDup,omitempty"`
	DVenc string  `json:"dVenc,omitempty"`
	VDup  float64 `json:"vDup,omitempty"`
}

// InfCteSub espelha CteSefazInfCteSub.
type InfCteSub struct {
	ChCte         string `json:"chCte"`
	IndAlteraToma int    `json:"indAlteraToma,omitempty"`
}

// InfGlobalizado espelha CteSefazInfGlobalizado.
type InfGlobalizado struct {
	XObs string `json:"xObs"`
}

// InfServVinc espelha CteSefazInfServVinc.
type InfServVinc struct {
	InfCTeMultimodal []InfCTeMultimodal `json:"infCTeMultimodal"`
}

// InfCTeMultimodal espelha CteSefazInfCTeMultimodal.
type InfCTeMultimodal struct {
	ChCTeMultimodal string `json:"chCTeMultimodal"`
}

// InfCteComp espelha CteSefazInfCteComp.
type InfCteComp struct {
	ChCTe string `json:"chCTe"`
}

// AutXML espelha CteSefazAutXML.
type AutXML struct {
	CNPJ string `json:"CNPJ,omitempty"`
	CPF  string `json:"CPF,omitempty"`
}

// RespTec espelha CteSefazRespTec.
type RespTec struct {
	CNPJ     string `json:"CNPJ"`
	XContato string `json:"xContato"`
	Email    string `json:"email"`
	Fone     string `json:"fone"`
	IdCSRT   int    `json:"idCSRT,omitempty"`
	CSRT     string `json:"CSRT,omitempty"`
	HashCSRT string `json:"hashCSRT,omitempty"`
}

// InfSolicNFF espelha CteSefazInfSolicNFF.
type InfSolicNFF struct {
	XSolic string `json:"xSolic"`
}

// InfCTeSupl espelha CteSefazInfCTeSupl.
type InfCTeSupl struct {
	QrCodCTe string `json:"qrCodCTe,omitempty"`
}
