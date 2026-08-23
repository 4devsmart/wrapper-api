package cte

import (
	"bufio"
	"strings"
)

// Emissao é o resultado estruturado de uma emissão de CT-e, extraído da
// resposta do ACBr (INI). cStat 100 = autorizado o uso do CT-e.
type Emissao struct {
	Sucesso   bool       `json:"sucesso"`
	Chave     string     `json:"chave,omitempty"`     // chCTe
	Protocolo string     `json:"protocolo,omitempty"` // nProt
	CStat     string     `json:"cstat,omitempty"`
	XMotivo   string     `json:"xmotivo,omitempty"`
	Recibo    string     `json:"recibo,omitempty"` // nRec (assíncrono)
	Erros     []Mensagem `json:"erros,omitempty"`
}

// Mensagem é um erro/alerta com código e descrição.
type Mensagem struct {
	Codigo    string `json:"codigo,omitempty"`
	Descricao string `json:"descricao,omitempty"`
}

// statusAutorizado são os cStat de sucesso na emissão do CT-e.
var statusAutorizado = map[string]bool{"100": true, "104": true}

// ParseEnvio interpreta a resposta INI do CTE_Enviar. Tolerante ao layout:
// varre as seções capturando chCTe/nProt/cStat/xMotivo/nRec onde aparecerem e
// coleta seções [ErroN]. Refinado contra a saída real da lib.
func ParseEnvio(resp string) Emissao {
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
		case "chCTe", "chave":
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
	e.Sucesso = statusAutorizado[e.CStat] && e.Chave != ""
	return e
}

// StatusEmissao mapeia a Emissao para o status do documento persistido.
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
