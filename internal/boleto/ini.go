package boleto

import (
	"strconv"

	"github.com/4devsmart/wrapper-api/internal/platform/inifmt"
)

// ToINI traduz o pedido para o INI combinado da ACBrLibBoleto:
// [Banco] + [Conta] + [Cedente] + [Titulo1..N]. Valores com VÍRGULA decimal e
// datas em DD/MM/YYYY (padrão do ACBr). Ver ModeloCedente_TitulosINI.
func ToINI(p Pedido) string {
	var b iniBuilder
	c := p.Conta

	b.Secao("Banco")
	b.KV("Numero", c.Banco)
	b.KVIntOpt("TipoCobranca", c.TipoCobranca)
	b.KVOpt("CNAB", c.CNAB)

	b.Secao("Conta")
	b.KV("Agencia", c.Agencia)
	b.KVOpt("DigitoAgencia", c.DigitoAgencia)
	b.KV("Conta", c.NumeroConta)
	b.KVOpt("DigitoConta", c.DigitoConta)

	b.Secao("Cedente")
	b.KV("Nome", c.Nome)
	b.KVOpt("FantasiaCedente", c.Fantasia)
	b.KV("CNPJCPF", c.CNPJCPF)
	b.KVIntOpt("TipoInscricao", c.TipoInscricao)
	b.KVIntOpt("TipoPessoa", c.TipoPessoa)
	b.KVOpt("Logradouro", c.Logradouro)
	b.KVOpt("Numero", c.Numero)
	b.KVOpt("Bairro", c.Bairro)
	b.KVOpt("Cidade", c.Cidade)
	b.KVOpt("CEP", c.CEP)
	b.KVOpt("Complemento", c.Complemento)
	b.KVOpt("UF", c.UF)
	b.KVOpt("Telefone", c.Telefone)
	b.KVOpt("CodigoCedente", c.CodigoCedente)
	b.KVOpt("Modalidade", c.Modalidade)
	b.KVOpt("Convenio", c.Convenio)
	b.KVOpt("CodTransmissao", c.CodTransmissao)
	b.KVIntOpt("TipoCarteira", c.TipoCarteira)
	b.KVIntOpt("TipoDocumento", c.TipoDocumento)
	b.KVIntOpt("RespEmis", c.RespEmis)
	b.KVIntOpt("LayoutBol", c.LayoutBol)
	if c.PixChave != "" {
		b.KV("PIX.TipoChavePix", strconv.Itoa(c.PixTipoChave))
		b.KV("PIX.Chave", c.PixChave)
	}

	// WS (registro online). ArquivoCRT/KEY (mTLS) são setados em runtime
	// pelo binding (gravados a partir do base64), não vão aqui.
	if ws := c.WS; ws != nil {
		b.Secao("BoletoWebSevice") // (typo é da própria ACBrLib)
		b.KVIntOpt("Ambiente", ws.Ambiente)
		b.Secao("BoletoCedenteWS")
		b.KVOpt("ClientID", ws.ClientID)
		b.KVOpt("ClientSecret", ws.ClientSecret)
		b.KVOpt("KeyUser", ws.KeyUser)
		b.KVOpt("Scope", ws.Scope)
		b.KVOpt("IndicadorPix", ws.IndicadorPix)
	}

	for i, t := range p.Titulos {
		b.Secao("Titulo" + strconv.Itoa(i+1))
		b.KV("NumeroDocumento", t.NumeroDocumento)
		b.KVOpt("SeuNumero", t.SeuNumero)
		b.KVOpt("NossoNumero", t.NossoNumero)
		b.KVOpt("Carteira", t.Carteira)
		b.KVOpt("Especie", t.Especie)
		b.KV("ValorDocumento", inifmt.Money(t.ValorDocumento))
		b.KV("Vencimento", inifmt.DataBR(t.Vencimento, b.Local()))
		b.KVOpt("DataDocumento", inifmt.DataBROpt(t.DataDocumento, b.Local()))
		b.KVOpt("Aceite", t.Aceite)
		b.KVOpt("Mensagem", t.Mensagem)
		for j, ins := range t.Instrucoes {
			if j >= 3 {
				break
			}
			b.KV("Instrucao"+strconv.Itoa(j+1), ins)
		}
		b.KVOpt("ValorMoraJuros", inifmt.MoneyOpt(t.ValorMoraJuros))
		b.KVOpt("CodigoMora", t.CodigoMora)
		b.KVOpt("PercentualMulta", inifmt.MoneyOpt(t.PercentualMulta))
		b.KVOpt("MultaValorFixo", inifmt.MoneyOpt(t.MultaValorFixo))
		if t.ValorDesconto != 0 {
			b.KV("ValorDesconto", inifmt.Money(t.ValorDesconto))
			b.KVOpt("DataDesconto", inifmt.DataBROpt(t.DataDesconto, b.Local()))
		}
		b.KVOpt("ValorAbatimento", inifmt.MoneyOpt(t.ValorAbatimento))
		b.KVOpt("ChaveNFe", t.ChaveNFe)

		s := t.Sacado
		b.KV("Sacado.NomeSacado", s.Nome)
		b.KV("Sacado.CNPJCPF", s.CNPJCPF)
		b.KVOpt("Sacado.Logradouro", s.Logradouro)
		b.KVOpt("Sacado.Numero", s.Numero)
		b.KVOpt("Sacado.Bairro", s.Bairro)
		b.KVOpt("Sacado.Cidade", s.Cidade)
		b.KVOpt("Sacado.UF", s.UF)
		b.KVOpt("Sacado.CEP", s.CEP)
		b.KVOpt("Sacado.Complemento", s.Complemento)
		b.KVOpt("Sacado.Email", s.Email)
	}
	return b.String()
}

// --- builder + helpers ------------------------------------------------------

// iniBuilder é o construtor compartilhado (internal/platform/inifmt) mais os
// métodos de DOMÍNIO deste documento, logo abaixo. O núcleo (seção, par
// chave=valor, datas no fuso do emitente) vive num lugar só: era o mesmo código
// nos quatro documentos, e um ajuste na sanitização precisava ser feito quatro
// vezes.
type iniBuilder struct{ inifmt.Builder }
