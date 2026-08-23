package boleto

import (
	"strconv"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// ToINI traduz o pedido para o INI combinado da ACBrLibBoleto:
// [Banco] + [Conta] + [Cedente] + [Titulo1..N]. Valores com VÍRGULA decimal e
// datas em DD/MM/YYYY (padrão do ACBr). Ver ModeloCedente_TitulosINI.
func ToINI(p Pedido) string {
	var b iniBuilder
	c := p.Conta

	b.section("Banco")
	b.kv("Numero", c.Banco)
	b.kvIntOpt("TipoCobranca", c.TipoCobranca)
	b.kvOpt("CNAB", c.CNAB)

	b.section("Conta")
	b.kv("Agencia", c.Agencia)
	b.kvOpt("DigitoAgencia", c.DigitoAgencia)
	b.kv("Conta", c.NumeroConta)
	b.kvOpt("DigitoConta", c.DigitoConta)

	b.section("Cedente")
	b.kv("Nome", c.Nome)
	b.kvOpt("FantasiaCedente", c.Fantasia)
	b.kv("CNPJCPF", c.CNPJCPF)
	b.kvIntOpt("TipoInscricao", c.TipoInscricao)
	b.kvIntOpt("TipoPessoa", c.TipoPessoa)
	b.kvOpt("Logradouro", c.Logradouro)
	b.kvOpt("Numero", c.Numero)
	b.kvOpt("Bairro", c.Bairro)
	b.kvOpt("Cidade", c.Cidade)
	b.kvOpt("CEP", c.CEP)
	b.kvOpt("Complemento", c.Complemento)
	b.kvOpt("UF", c.UF)
	b.kvOpt("Telefone", c.Telefone)
	b.kvOpt("CodigoCedente", c.CodigoCedente)
	b.kvOpt("Modalidade", c.Modalidade)
	b.kvOpt("Convenio", c.Convenio)
	b.kvOpt("CodTransmissao", c.CodTransmissao)
	b.kvIntOpt("TipoCarteira", c.TipoCarteira)
	b.kvIntOpt("TipoDocumento", c.TipoDocumento)
	b.kvIntOpt("RespEmis", c.RespEmis)
	b.kvIntOpt("LayoutBol", c.LayoutBol)
	if c.PixChave != "" {
		b.kv("PIX.TipoChavePix", strconv.Itoa(c.PixTipoChave))
		b.kv("PIX.Chave", c.PixChave)
	}

	// WS (registro online). ArquivoCRT/KEY (mTLS) são setados em runtime
	// pelo binding (gravados a partir do base64) — não vão aqui.
	if ws := c.WS; ws != nil {
		b.section("BoletoWebSevice") // (typo é da própria ACBrLib)
		b.kvIntOpt("Ambiente", ws.Ambiente)
		b.section("BoletoCedenteWS")
		b.kvOpt("ClientID", ws.ClientID)
		b.kvOpt("ClientSecret", ws.ClientSecret)
		b.kvOpt("KeyUser", ws.KeyUser)
		b.kvOpt("Scope", ws.Scope)
		b.kvOpt("IndicadorPix", ws.IndicadorPix)
	}

	for i, t := range p.Titulos {
		b.section("Titulo" + strconv.Itoa(i+1))
		b.kv("NumeroDocumento", t.NumeroDocumento)
		b.kvOpt("SeuNumero", t.SeuNumero)
		b.kvOpt("NossoNumero", t.NossoNumero)
		b.kvOpt("Carteira", t.Carteira)
		b.kvOpt("Especie", t.Especie)
		b.kv("ValorDocumento", money(t.ValorDocumento))
		b.kv("Vencimento", dateBR(t.Vencimento))
		b.kvOpt("DataDocumento", dateBROpt(t.DataDocumento))
		b.kvOpt("Aceite", t.Aceite)
		b.kvOpt("Mensagem", t.Mensagem)
		for j, ins := range t.Instrucoes {
			if j >= 3 {
				break
			}
			b.kv("Instrucao"+strconv.Itoa(j+1), ins)
		}
		b.kvOpt("ValorMoraJuros", moneyOpt(t.ValorMoraJuros))
		b.kvOpt("CodigoMora", t.CodigoMora)
		b.kvOpt("PercentualMulta", moneyOpt(t.PercentualMulta))
		b.kvOpt("MultaValorFixo", moneyOpt(t.MultaValorFixo))
		if t.ValorDesconto != 0 {
			b.kv("ValorDesconto", money(t.ValorDesconto))
			b.kvOpt("DataDesconto", dateBROpt(t.DataDesconto))
		}
		b.kvOpt("ValorAbatimento", moneyOpt(t.ValorAbatimento))
		b.kvOpt("ChaveNFe", t.ChaveNFe)

		s := t.Sacado
		b.kv("Sacado.NomeSacado", s.Nome)
		b.kv("Sacado.CNPJCPF", s.CNPJCPF)
		b.kvOpt("Sacado.Logradouro", s.Logradouro)
		b.kvOpt("Sacado.Numero", s.Numero)
		b.kvOpt("Sacado.Bairro", s.Bairro)
		b.kvOpt("Sacado.Cidade", s.Cidade)
		b.kvOpt("Sacado.UF", s.UF)
		b.kvOpt("Sacado.CEP", s.CEP)
		b.kvOpt("Sacado.Complemento", s.Complemento)
		b.kvOpt("Sacado.Email", s.Email)
	}
	return b.String()
}

// --- builder + helpers ------------------------------------------------------

type iniBuilder struct{ sb strings.Builder }

func (b *iniBuilder) section(name string) {
	if b.sb.Len() > 0 {
		b.sb.WriteByte('\n')
	}
	b.sb.WriteString("[" + name + "]\n")
}

// sanitizeINIVal neutraliza CR/LF no valor (evita injeção de chave/seção INI via
// campo de texto livre do cliente que a ACBrLib consumiria).
func sanitizeINIVal(s string) string { return inifmt.Sanitize(s) }

func (b *iniBuilder) kv(key, val string) { b.sb.WriteString(key + "=" + sanitizeINIVal(val) + "\n") }
func (b *iniBuilder) kvOpt(key, val string) {
	if val != "" {
		b.kv(key, val)
	}
}
func (b *iniBuilder) kvIntOpt(key string, v int) {
	if v != 0 {
		b.kv(key, strconv.Itoa(v))
	}
}
func (b *iniBuilder) String() string { return b.sb.String() }

func money(v float64) string    { return inifmt.Money(v) }
func moneyOpt(v float64) string { return inifmt.MoneyOpt(v) }
func dateBR(s string) string {
	if s == "" {
		return time.Now().Format("02/01/2006")
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "02/01/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02/01/2006")
		}
	}
	return s
}
func dateBROpt(s string) string {
	if s == "" {
		return ""
	}
	return dateBR(s)
}
