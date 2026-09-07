// Package lockstep compara o que a ACBrLib LÊ de um INI com o que os nossos
// construtores ESCREVEM nele, nas duas direções.
//
// Existe porque a mesma classe de bug já apareceu cinco vezes: um campo é
// aceito pelo contrato, a emissão parece funcionar, e o dado some antes do XML.
// A última foi o grupo de retenções federais da NFS-e, escrito na seção
// [tribFed] quando a biblioteca lê [tribFederal]: nome parecido, grupo inteiro
// descartado em silêncio.
//
// As duas pontas eram aproximações, e é por isso que aquele caso passou:
//
//   - o que ESCREVEMOS era raspado do fonte Go por expressão regular, que não
//     via seção montada por concatenação nem voltava ao normal na fronteira de
//     função. Hoje vem de espelho.SecoesEChaves, que roda o construtor e lê o
//     INI que saiu;
//   - o que a biblioteca LÊ era um TSV feito à mão, sem gerador, com três
//     classes de erro: chave de fallback ausente (o segundo argumento de um
//     ReadString aninhado), nome corrompido por achatamento de índice (capM3
//     virava capM, que não existe) e chave atribuída à seção errada (onze
//     chaves de [prop] arquivadas em [veic]). Hoje vem de
//     scripts/gerar-chaves-lerini.py, direto do fonte Pascal pinado.
//
// Com as duas pontas mecânicas, some também o palpite do meio: não é mais
// preciso adivinhar se o dígito no fim de uma seção é índice ([det001]) ou
// nome ([ICMS60], [toma4]). O gerador marca a indexada com "*", porque no fonte
// as duas formas são distinguíveis (concatenação com IntToStrZero contra
// literal), e aqui só se obedece a marca.
package lockstep

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Entrada é um par (seção, chave) que a biblioteca aceita.
type Entrada struct {
	Secao    string // sem o "*"; forma canônica, como no fonte
	Chave    string // idem
	Indexada bool   // a seção recebe índice: [det001], [det002]...
	// ChaveIndexada vale para a chave montada por concatenação. É rara: existe
	// uma só em todo o fonte, o cInfManu do aéreo no CT-e, escrita como
	// cInfManu1, cInfManu2... Tratá-la como literal acusava lacuna que não é.
	ChaveIndexada bool
}

// ID é a forma "secao/chave" usada em listas de exceção e em mensagens.
func (e Entrada) ID() string { return e.Secao + "/" + e.Chave }

// Chave de busca em listas escritas à mão, insensível a caixa.
func (e Entrada) IDLower() string { return strings.ToLower(e.ID()) }

// Snapshot é o conjunto de pares que a biblioteca lê, vindo do TSV gerado.
type Snapshot struct {
	entradas []Entrada
	// exatas indexa as seções não indexadas; prefixos, as indexadas. Ambos em
	// minúsculas: o TIniFile do ACBr é case-insensitive, e comparar sensível
	// acusaria "Exped" contra "exped" como divergência, que é ruído.
	exatas   map[string]map[string]bool
	prefixos map[string]map[string]bool
	// chavesIdx: seção (sem índice) -> prefixos de chave indexada.
	chavesIdx map[string]map[string]bool
}

var reIndice = regexp.MustCompile(`[0-9]+$`)

// Carregar lê o TSV gerado por scripts/gerar-chaves-lerini.py.
func Carregar(caminho string) (*Snapshot, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("snapshot ausente: %w", err)
	}
	s := &Snapshot{
		exatas:    map[string]map[string]bool{},
		prefixos:  map[string]map[string]bool{},
		chavesIdx: map[string]map[string]bool{},
	}
	for n, linha := range strings.Split(string(b), "\n") {
		linha = strings.TrimRight(linha, "\r")
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		campos := strings.Split(linha, "\t")
		if len(campos) != 2 || campos[0] == "" || campos[1] == "" {
			return nil, fmt.Errorf("%s:%d: linha malformada: %q", caminho, n+1, linha)
		}
		secao, chave := campos[0], campos[1]
		indexada := strings.HasSuffix(secao, "*")
		secao = strings.TrimSuffix(secao, "*")
		chaveIdx := strings.HasSuffix(chave, "*")
		chave = strings.TrimSuffix(chave, "*")
		s.entradas = append(s.entradas, Entrada{
			Secao: secao, Chave: chave, Indexada: indexada, ChaveIndexada: chaveIdx,
		})
		if chaveIdx {
			if s.chavesIdx[strings.ToLower(secao)] == nil {
				s.chavesIdx[strings.ToLower(secao)] = map[string]bool{}
			}
			s.chavesIdx[strings.ToLower(secao)][strings.ToLower(chave)] = true
		}

		alvo := s.exatas
		if indexada {
			alvo = s.prefixos
		}
		k := strings.ToLower(secao)
		if alvo[k] == nil {
			alvo[k] = map[string]bool{}
		}
		alvo[k][strings.ToLower(chave)] = true
	}
	if len(s.entradas) == 0 {
		return nil, fmt.Errorf("%s: nenhuma entrada", caminho)
	}
	return s, nil
}

// Entradas devolve os pares aceitos, ordenados, para varrer a direção "a
// biblioteca aceita e nós não enviamos".
func (s *Snapshot) Entradas() []Entrada {
	out := append([]Entrada(nil), s.entradas...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Secao != out[j].Secao {
			return out[i].Secao < out[j].Secao
		}
		return out[i].Chave < out[j].Chave
	})
	return out
}

// Aceita responde se a biblioteca lê essa chave numa seção ESCRITA POR NÓS,
// isto é, com o índice já no nome ([det001]) quando for o caso.
//
// A seção indexada casa pelo prefixo, e só ela: [ICMS60] não é [ICMS] com
// índice 60, e tratar assim achatava as sete variantes de ICMS numa só. Bastava
// enviar uma chave em [ICMS20] para o teste dar por enviada a mesma chave em
// [ICMS60], e foi assim que a desoneração faltando em ICMS60 e ICMS90 passou
// meses despercebida.
func (s *Snapshot) Aceita(secaoEscrita, chave string) bool {
	sec, ch := strings.ToLower(secaoEscrita), strings.ToLower(chave)
	if s.exatas[sec][ch] {
		return true
	}
	// Só tenta prefixo se houver mesmo um índice a tirar: sem isto, uma seção
	// não indexada casaria com o prefixo homônimo.
	base := reIndice.ReplaceAllString(sec, "")
	if base != sec && s.prefixos[base][ch] {
		return true
	}
	// Chave indexada: cInfManu1 casa o prefixo cInfManu.
	if bch := reIndice.ReplaceAllString(ch, ""); bch != ch {
		if s.chavesIdx[sec][bch] || s.chavesIdx[base][bch] {
			return true
		}
	}
	return false
}

// Escritas é o que um construtor de fato escreveu, por seção, com o índice no
// nome. Vem de espelho.SecoesEChaves.
type Escritas map[string]map[string]bool

// Uniao junta o que vários construtores escrevem. A NFS-e tem dois sobre o
// mesmo modelo (Padrão Nacional e ABRASF), e uma chave escrita por qualquer um
// deles chega à biblioteca.
func Uniao(conjuntos ...Escritas) Escritas {
	out := Escritas{}
	for _, c := range conjuntos {
		for secao, chaves := range c {
			if out[secao] == nil {
				out[secao] = map[string]bool{}
			}
			for k := range chaves {
				out[secao][k] = true
			}
		}
	}
	return out
}

// Enviamos responde se alguma seção nossa satisfaz a entrada da biblioteca.
// Para entrada indexada, qualquer [Nome<dígitos>] serve.
func (e Escritas) Enviamos(ent Entrada) bool {
	alvo, chave := strings.ToLower(ent.Secao), strings.ToLower(ent.Chave)
	for secao, chaves := range e {
		s := strings.ToLower(secao)
		casa := s == alvo
		if !casa && ent.Indexada {
			base := reIndice.ReplaceAllString(s, "")
			casa = base == alvo && base != s
		}
		if !casa {
			continue
		}
		for k := range chaves {
			lk := strings.ToLower(k)
			if lk == chave {
				return true
			}
			if ent.ChaveIndexada && reIndice.ReplaceAllString(lk, "") == chave && lk != chave {
				return true
			}
		}
	}
	return false
}

// Mortas lista o que ESCREVEMOS e a biblioteca não lê: o dado é aceito e
// descartado em silêncio, que é a direção mais cara das duas.
func (e Escritas) Mortas(s *Snapshot) []string {
	var out []string
	for secao, chaves := range e {
		for chave := range chaves {
			if !s.Aceita(secao, chave) {
				out = append(out, secao+"/"+chave)
			}
		}
	}
	sort.Strings(out)
	return out
}

// CarregarBaseline lê o TSV de lacunas JÁ CONHECIDAS (primeira coluna
// "secao/chave", segunda o motivo). O teste não cobra o passado: cobra o que
// aparecer depois, que é onde mora o campo esquecido de verdade.
func CarregarBaseline(caminho string) (map[string]bool, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("baseline ausente: %w", err)
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		// Minúsculas: o INI do ACBr é case-insensitive, e o baseline foi escrito
		// à mão ao longo do tempo, com "Valores/vTPrest" e "valores/vtprest"
		// convivendo. Comparar sensível acusaria linha morta que não é.
		if campo := strings.SplitN(l, "\t", 2)[0]; strings.Contains(campo, "/") {
			out[strings.ToLower(campo)] = true
		}
	}
	return out, nil
}
