package cte

import "strconv"

// CT-e Simplificado (tpCTe=5): modelo 57 com corpo próprio, tomador único
// ([toma]), detalhamento por trecho ([detNNN]) e totais ([total]), em vez de
// rem/dest/vPrest do CT-e Normal. Ver ModeloCTeSimplificadoINI.html e o
// Ler_CTeSimp do ACBrCTe.IniReader. A Reforma Tributária (IBSCBS) é a mesma do
// Normal (reusa imposto()). tpCTe: 5=CTeSimp, 6=Substituto Simplificado.
const (
	TpCTeSimp       = 5
	TpCTeSubstSimpl = 6
)

// PedidoSimp espelha o pedido de emissão do CT-e Simplificado (contrato
// ACBr.API: mesmo envelope do CT-e, corpo simplificado).
type PedidoSimp struct {
	InfCte     InfCteSimp  `json:"infCte"`
	InfCTeSupl *InfCTeSupl `json:"infCTeSupl,omitempty"`
	Ambiente   string      `json:"ambiente" enum:"Ambiente"`
	Referencia string      `json:"referencia,omitempty"`
}

// InfCteSimp é o corpo do CT-e Simplificado.
type InfCteSimp struct {
	Versao     string     `json:"versao"`
	Id         string     `json:"Id,omitempty"`
	Ide        Ide        `json:"ide"`
	Compl      *Compl     `json:"compl,omitempty"`
	Emit       Emit       `json:"emit"`
	Toma       Toma       `json:"toma" enum:"Tomador"`
	InfCarga   InfCarga   `json:"infCarga"`
	Det        []Det      `json:"det"`
	Imp        Imp        `json:"imp"`
	InfModal   InfModal   `json:"infModal"`
	Cobr       *Cobr      `json:"cobr,omitempty"`
	InfCteSub  *InfCteSub `json:"infCteSub,omitempty"`
	Total      Total      `json:"total"`
	InfRespTec *RespTec   `json:"infRespTec,omitempty"`
}

// Toma é o tomador único do CT-e Simplificado/OS (seção [toma]).
type Toma struct {
	Toma      int      `json:"toma" enum:"Tomador"`
	IndIEToma int      `json:"indIEToma,omitempty" enum:"IndicadorIEToma"`
	CNPJ      string   `json:"CNPJ,omitempty"`
	CPF       string   `json:"CPF,omitempty"`
	IE        string   `json:"IE,omitempty"`
	XNome     string   `json:"xNome,omitempty"`
	XFant     string   `json:"xFant,omitempty"`
	ISUF      string   `json:"ISUF,omitempty"`
	Email     string   `json:"email,omitempty"`
	Fone      string   `json:"fone,omitempty"`
	EnderToma Endereco `json:"enderToma"`
}

// Det é um trecho do detalhamento ([detNNN]): origem/destino, valores e os
// documentos transportados (componentes, NF-e e documentos anteriores).
type Det struct {
	CMunIni   string      `json:"cMunIni"`
	XMunIni   string      `json:"xMunIni,omitempty"`
	CMunFim   string      `json:"cMunFim"`
	XMunFim   string      `json:"xMunFim,omitempty"`
	VPrest    float64     `json:"vPrest"`
	VRec      float64     `json:"vRec"`
	Comp      []Comp      `json:"comp,omitempty"`
	InfNFe    []InfNFe    `json:"infNFe,omitempty"`
	InfDocAnt []DetDocAnt `json:"infDocAnt,omitempty"`
}

// DetDocAnt é um documento anterior dentro de um trecho do Simplificado.
type DetDocAnt struct {
	ChCTe               string          `json:"chCTe"`
	TpPrest             int             `json:"tpPrest,omitempty" enum:"TipoPrestacaoAnterior"`
	InfNFeTranspParcial []TranspParcial `json:"infNFeTranspParcial,omitempty"`
}

// TranspParcial é uma NF-e de transporte parcial vinculada a um doc anterior.
type TranspParcial struct {
	ChNFe string `json:"chNFe"`
}

// Total são os totais do CT-e Simplificado (seção [total]).
type Total struct {
	VTPrest float64 `json:"vTPrest"`
	VTRec   float64 `json:"vTRec"`
}

// ToINISimp traduz o pedido do CT-e Simplificado para o INI (CTE_CarregarINI).
// Força tpCTe=5 (CTeSimp) quando não informado, para a lib acionar Ler_CTeSimp.
func ToINISimp(p PedidoSimp) string {
	inf := p.InfCte
	if inf.Ide.TpCTe == 0 {
		inf.Ide.TpCTe = TpCTeSimp
	}
	var b iniBuilder

	b.section("infCTe")
	b.kv("versao", defaultStr(inf.Versao, "4.00"))

	b.identificacao(inf.Ide, p.Ambiente)
	b.complemento(inf.Compl)
	b.emitente(inf.Emit)
	b.tomador(inf.Toma)
	b.infCargaQ(inf.InfCarga)
	b.detalhamento(inf.Det)
	b.imposto(inf.Imp)
	b.modalRodo(inf.InfModal.Rodo)
	b.cobranca(inf.Cobr)
	b.subst(inf.InfCteSub)

	b.section("total")
	b.kv("vTPrest", money(inf.Total.VTPrest))
	b.kv("vTRec", money(inf.Total.VTRec))

	b.respTec(inf.InfRespTec)
	return b.String()
}

// tomador emite a seção [toma] (tomador único do Simplificado/OS).
func (b *iniBuilder) tomador(t Toma) {
	b.section("toma")
	b.kv("toma", strconv.Itoa(t.Toma))
	b.kvIntOpt("indIEToma", t.IndIEToma)
	b.kvOpt("CNPJCPF", firstNonEmpty(t.CNPJ, t.CPF))
	b.kvOpt("IE", t.IE)
	b.kvOpt("xNome", t.XNome)
	b.kvOpt("xFant", t.XFant)
	b.kvOpt("ISUF", t.ISUF)
	b.kvOpt("email", t.Email)
	b.kvOpt("fone", t.Fone)
	b.endereco(t.EnderToma)
}

// detalhamento emite [detNNN] + aninhados [CompNNNmmm]/[infNFeNNNmmm]/
// [infDocAntNNNmmm] (+ [infNFeTranspParcialNNNmmmkkk]).
func (b *iniBuilder) detalhamento(dets []Det) {
	for i, d := range dets {
		di := seq(i + 1)
		b.section("det" + di)
		b.kv("cMunIni", d.CMunIni)
		b.kvOpt("xMunIni", d.XMunIni)
		b.kv("cMunFim", d.CMunFim)
		b.kvOpt("xMunFim", d.XMunFim)
		b.kv("vPrest", money(d.VPrest))
		b.kv("vRec", money(d.VRec))
		for j, c := range d.Comp {
			b.section("Comp" + di + seq(j+1))
			b.kv("xNome", c.XNome)
			b.kv("vComp", money(c.VComp))
		}
		for j, nfe := range d.InfNFe {
			b.section("infNFe" + di + seq(j+1))
			b.kv("chave", nfe.Chave)
		}
		for j, da := range d.InfDocAnt {
			dj := di + seq(j+1)
			b.section("infDocAnt" + dj)
			b.kv("chCTe", da.ChCTe)
			b.kvIntOpt("tpPrest", da.TpPrest)
			for k, tp := range da.InfNFeTranspParcial {
				b.section("infNFeTranspParcial" + dj + seq(k+1))
				b.kv("chNFe", tp.ChNFe)
			}
		}
	}
}
