package nfse

import (
	"bufio"
	"strings"
)

// lerRespostaINI percorre a resposta INI da lib chamando campo() para cada par
// (seção, chave, valor), e devolve os blocos [ErroN]/[AlertaN] já montados.
//
// As respostas de emissão, cancelamento e evento têm a mesma forma: uma seção
// de resultado, com nome diferente em cada uma, e as mesmas listas de mensagens
// ao lado. Só a seção de resultado é assunto de cada parser; o resto era o mesmo
// laço escrito três vezes.
//
// Resposta que NÃO é INI vira um erro único com o texto cru. É o formato que a
// lib usa quando falha antes de montar a resposta, e é por ali que chega o
// "Serviço %s não implementado para este provedor" (ERR_NAO_IMP).
func lerRespostaINI(resp string, campo func(secao, chave, valor string)) (erros, alertas []Mensagem) {
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return nil, nil
	}
	if !strings.HasPrefix(resp, "[") {
		return []Mensagem{{Descricao: resp}}, nil
	}

	var secao string
	var msg Mensagem
	flush := func() {
		vazia := msg.Codigo == "" && msg.Descricao == ""
		switch {
		case vazia:
		case strings.HasPrefix(secao, "Erro"):
			erros = append(erros, msg)
		case strings.HasPrefix(secao, "Alerta"):
			alertas = append(alertas, msg)
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

		if strings.HasPrefix(secao, "Erro") || strings.HasPrefix(secao, "Alerta") {
			switch key {
			case "Codigo":
				msg.Codigo = val
			case "Descricao":
				msg.Descricao = val
			}
			continue
		}
		campo(secao, key, val)
	}
	flush()
	return erros, alertas
}
