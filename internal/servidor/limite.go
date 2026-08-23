package servidor

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Limite de requisições por endereço, e a descoberta de qual é o endereço.
//
// Existe pelo mesmo motivo do teto de corpo: esta API recebe um Bearer no
// cabeçalho e um certificado no payload. Sem limite, tentar tokens sai ao ritmo
// da rede, e uma enxurrada de transmissões ocupa todos os slots de worker, que
// são poucos por desenho (o default é UM). Não substitui o limite do proxy de
// borda: é o piso que vale mesmo quando não há proxy nenhum na frente.

// maxClientes limita quantos endereços o limitador acompanha ao mesmo tempo. O
// mapa é indexado por endereço do chamador, ou seja, por dado que o atacante
// escolhe: sem teto, acompanhar seria o próprio vetor de exaustão de memória
// que o limite deveria conter.
const maxClientes = 8192

// balde é o saldo de um cliente. A ficha é reposta a taxa constante e acumula
// até o teto: uma rajada curta passa, um fluxo contínuo acima da taxa não.
type balde struct {
	fichas float64
	visto  time.Time
}

type limitador struct {
	porMinuto int
	teto      float64
	agora     func() time.Time // injetável: teste de taxa não pode depender de dormir

	mu        sync.Mutex
	clientes  map[string]*balde
	avisadoEm time.Time
}

// novoLimitador devolve nil quando o limite está desligado (0 ou negativo), e o
// middleware some da cadeia. Desligar é opção legítima: atrás de um gateway que
// já limita, contar duas vezes só confunde o diagnóstico.
func novoLimitador(porMinuto int) *limitador {
	if porMinuto <= 0 {
		return nil
	}
	return &limitador{
		porMinuto: porMinuto,
		teto:      float64(porMinuto),
		agora:     time.Now,
		clientes:  make(map[string]*balde),
	}
}

// permitir consome uma ficha do cliente. Quando recusa, devolve quanto falta
// para a próxima ficha: é o Retry-After que o chamador precisa respeitar.
func (l *limitador) permitir(ip string) (bool, time.Duration) {
	porSegundo := float64(l.porMinuto) / 60
	agora := l.agora()

	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.clientes[ip]
	if b == nil {
		if len(l.clientes) >= maxClientes && !l.abrirEspaco(agora) {
			// Cheio mesmo depois da limpeza: enxurrada de endereços distintos.
			// Deixa passar em vez de recusar, porque recusar transformaria o
			// limitador na negação de serviço que ele existe para evitar. Quem
			// já estava sendo acompanhado continua limitado.
			l.avisarCheio(agora)
			return true, 0
		}
		l.clientes[ip] = &balde{fichas: l.teto - 1, visto: agora}
		return true, 0
	}

	b.fichas = min(b.fichas+agora.Sub(b.visto).Seconds()*porSegundo, l.teto)
	b.visto = agora
	if b.fichas < 1 {
		return false, time.Duration((1 - b.fichas) / porSegundo * float64(time.Second))
	}
	b.fichas--
	return true, 0
}

// abrirEspaco descarta quem já repôs todas as fichas. Um balde cheio não guarda
// informação nenhuma: acompanhar ou esquecer dá exatamente o mesmo resultado na
// próxima requisição daquele endereço.
func (l *limitador) abrirEspaco(agora time.Time) bool {
	porSegundo := float64(l.porMinuto) / 60
	for ip, b := range l.clientes {
		if b.fichas+agora.Sub(b.visto).Seconds()*porSegundo >= l.teto {
			delete(l.clientes, ip)
		}
	}
	return len(l.clientes) < maxClientes
}

// avisarCheio registra no máximo uma vez por minuto: o aviso acontece
// justamente quando há tráfego demais, e logar por requisição seria juntar
// fome com vontade de comer.
func (l *limitador) avisarCheio(agora time.Time) {
	if agora.Sub(l.avisadoEm) < time.Minute {
		return
	}
	l.avisadoEm = agora
	slog.Warn("limite por IP suspenso para endereços novos: teto de clientes acompanhados atingido",
		"clientes", len(l.clientes), "dica", "o limite por IP não substitui o limite no proxy de borda")
}

// --- middleware -------------------------------------------------------------

// limitarTaxa recusa com 429 quem passa de API_RATE_PER_MIN. Fica FORA da
// autenticação de propósito: se só valesse depois do Bearer, tentar tokens
// continuaria saindo ao ritmo da rede, que é o ataque que mais importa aqui.
func (s *Servidor) limitarTaxa(next http.Handler) http.Handler {
	if s.taxa == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sonda de orquestrador não gasta ficha: ela chega do mesmo endereço a
		// cada poucos segundos, e limitá-la derrubaria o container são, que é
		// o mesmo motivo de ela não exigir token.
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		ok, espera := s.taxa.permitir(s.ipDoCliente(r))
		if ok {
			next.ServeHTTP(w, r)
			return
		}
		segundos := max(int((espera + time.Second - 1).Seconds()), 1)
		w.Header().Set("Retry-After", strconv.Itoa(segundos))
		escreverErro(w, http.StatusTooManyRequests, "limite_de_requisicoes",
			"limite de "+strconv.Itoa(s.taxa.porMinuto)+" requisições por minuto excedido para este endereço")
	})
}

// --- endereço do chamador ---------------------------------------------------

// ipDoCliente devolve o endereço do chamador, que é a chave do limite.
//
// X-Forwarded-For é cabeçalho: quem chama escreve nele o que quiser. Por isso
// ele só vale quando o peer da conexão é infraestrutura interna, ou seja,
// quando existe mesmo um proxy nosso na frente. Numa exposição direta o
// cabeçalho forjado é ignorado e vale o endereço da conexão, senão bastaria um
// XFF novo por requisição para o limite não valer nada.
func (s *Servidor) ipDoCliente(r *http.Request) string {
	peer := semPorta(r.RemoteAddr)
	if !s.cfg.TrustProxyHeaders || !interno(peer) {
		return peer
	}
	if ip := clienteNaCadeia(r.Header.Values("X-Forwarded-For")); ip != "" {
		return ip
	}
	// X-Real-IP é um endereço só, escrito pelo proxy imediato (é o que o nginx
	// põe): não há cadeia para percorrer.
	if ip, err := netip.ParseAddr(semPorta(r.Header.Get("X-Real-IP"))); err == nil {
		return ip.Unmap().String()
	}
	return peer
}

// clienteNaCadeia lê X-Forwarded-For da DIREITA para a esquerda e devolve o
// primeiro endereço externo.
//
// Cada proxy acrescenta ao FIM da lista, então a direita é a parte que a nossa
// infraestrutura escreveu e a esquerda é a parte que o chamador pode ter
// inventado. Pegar o primeiro da lista, que é o costume, entregaria a chave do
// limite ao atacante: ele manda "X-Forwarded-For: 1.2.3.4" diferente a cada
// requisição e nunca esbarra no teto.
func clienteNaCadeia(cabecalhos []string) string {
	var cadeia []string
	for _, c := range cabecalhos {
		for _, p := range strings.Split(c, ",") {
			if p = strings.TrimSpace(p); p != "" {
				cadeia = append(cadeia, p)
			}
		}
	}
	maisAEsquerda := ""
	for i := len(cadeia) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(semPorta(cadeia[i]))
		if err != nil {
			continue
		}
		normal := ip.Unmap().String()
		maisAEsquerda = normal
		if !interno(normal) {
			return normal
		}
	}
	// Cadeia inteira interna: o chamador é interno mesmo, e vale quem originou.
	return maisAEsquerda
}

// interno diz se o endereço é de infraestrutura (loopback, rede privada,
// link-local). É o que separa "tem um proxy nosso na frente" de "alguém
// conectou direto e escreveu o cabeçalho".
func interno(ip string) bool {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	a = a.Unmap()
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsUnspecified()
}

// semPorta aceita tanto "ip:porta" e "[v6]:porta" quanto o endereço puro: o
// RemoteAddr sempre traz porta, o cabeçalho quase nunca.
func semPorta(addr string) string {
	addr = strings.TrimSpace(addr)
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return strings.Trim(addr, "[]")
}
