package fiscal

import (
	"bufio"
	"slices"
	"strings"
)

// Emissao é o resultado de uma emissão de CT-e ou MDF-e, extraído da resposta
// INI da lib.
type Emissao struct {
	Sucesso   bool       `json:"sucesso"`
	Chave     string     `json:"chave,omitempty"`
	Protocolo string     `json:"protocolo,omitempty"` // nProt
	CStat     string     `json:"cstat,omitempty"`
	XMotivo   string     `json:"xmotivo,omitempty"`
	Recibo    string     `json:"recibo,omitempty"` // nRec
	Erros     []Mensagem `json:"erros,omitempty"`
}

// Mensagem é um erro ou alerta com código e descrição.
type Mensagem struct {
	Codigo    string `json:"codigo,omitempty"`
	Descricao string `json:"descricao,omitempty"`
}

// ParseEnvio interpreta a resposta INI do envio. É tolerante ao layout: varre as
// seções capturando chave, nProt, cStat, xMotivo e nRec onde aparecerem, e coleta
// as seções [ErroN].
//
// Só duas coisas são do documento, e por isso vêm de quem chama: o nome da tag da
// chave e os cStat de sucesso. As listas NÃO são iguais entre os documentos, e
// juntá-las aceitaria como autorizado um código de outro documento.
func ParseEnvio(resp, tagChave string, autorizados ...string) Emissao {
	resp = strings.TrimSpace(resp)
	var e Emissao
	if resp == "" {
		return e
	}
	if !strings.HasPrefix(resp, "[") {
		e.Erros = append(e.Erros, Mensagem{Descricao: resp})
		return e
	}

	var secao string
	var msg Mensagem
	flush := func() {
		if strings.HasPrefix(secao, "Erro") && (msg.Codigo != "" || msg.Descricao != "") {
			e.Erros = append(e.Erros, msg)
		}
		msg = Mensagem{}
	}

	sc := bufio.NewScanner(strings.NewReader(resp))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			secao = line[1 : len(line)-1]
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)

		if strings.HasPrefix(secao, "Erro") {
			switch key {
			case "Codigo":
				msg.Codigo = val
			case "Descricao":
				msg.Descricao = val
			}
			continue
		}
		switch key {
		case tagChave, "chave":
			if val != "" {
				e.Chave = val
			}
		case "nProt", "protocolo":
			if val != "" {
				e.Protocolo = val
			}
		case "cStat":
			e.CStat = val
		case "xMotivo":
			e.XMotivo = val
		case "nRec":
			e.Recibo = val
		}
	}
	flush()
	e.Sucesso = slices.Contains(autorizados, e.CStat) && e.Chave != ""
	return e
}

// StatusEmissao traduz a Emissao no status devolvido ao cliente.
func StatusEmissao(e Emissao) string {
	switch {
	case e.Sucesso:
		return "autorizado"
	case len(e.Erros) > 0 || e.CStat != "":
		return "rejeitado"
	default:
		return "erro"
	}
}
