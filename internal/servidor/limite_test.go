package servidor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/acbr"
)

// O limite por IP é a última rede antes do worker: os slots são poucos por
// desenho (o default é UM), e o Bearer é um segredo só. Um teste que dependesse
// de dormir seria lento e instável, então o relógio é injetado.

// relogio devolve o servidor e um ponteiro para o instante corrente.
func relogio(t *testing.T, porMinuto int, confiaNoProxy bool) (*Servidor, *time.Time) {
	t.Helper()
	cfg := cfgTeste()
	cfg.APIRatePerMin = porMinuto
	cfg.TrustProxyHeaders = confiaNoProxy
	s := Novo(cfg, &acbr.Servicos{})
	agora := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	if s.taxa != nil {
		s.taxa.agora = func() time.Time { return agora }
	}
	return s, &agora
}

func pedir(h http.Handler, rota, ip string, cab map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, rota, nil)
	r.RemoteAddr = ip
	r.Header.Set("Authorization", "Bearer s3gr3d0")
	for k, v := range cab {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestLimitePorIP_EstouraEDepoisRepoe(t *testing.T) {
	s, agora := relogio(t, 3, false)
	h := s.Handler()

	for i := 1; i <= 3; i++ {
		if rec := pedir(h, "/v1/capacidades", "203.0.113.7:1111", nil); rec.Code != http.StatusOK {
			t.Fatalf("requisição %d = %d, ainda estava dentro do teto", i, rec.Code)
		}
	}

	rec := pedir(h, "/v1/capacidades", "203.0.113.7:1111", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a quarta requisição = %d, quero 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "limite_de_requisicoes") {
		t.Errorf("o corpo precisa trazer o código estável: %s", rec.Body)
	}
	// Sem Retry-After o cliente não sabe quando voltar e tenta em loop, que é
	// exatamente o tráfego que o limite queria conter.
	espera, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || espera < 1 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}

	// A ficha volta com o tempo: a 3 por minuto, uma a cada 20 segundos.
	*agora = agora.Add(20 * time.Second)
	if rec := pedir(h, "/v1/capacidades", "203.0.113.7:1111", nil); rec.Code != http.StatusOK {
		t.Errorf("depois da reposição = %d, quero 200", rec.Code)
	}
}

// Cada endereço tem o seu saldo: um cliente barulhento não pode derrubar o
// vizinho, senão o limite vira ferramenta de ataque em vez de defesa.
func TestLimitePorIP_UmNaoAfetaOOutro(t *testing.T) {
	s, _ := relogio(t, 2, false)
	h := s.Handler()

	for range 3 {
		pedir(h, "/v1/capacidades", "203.0.113.7:1111", nil)
	}
	if rec := pedir(h, "/v1/capacidades", "203.0.113.8:2222", nil); rec.Code != http.StatusOK {
		t.Errorf("o segundo endereço = %d, quero 200", rec.Code)
	}
}

func TestLimitePorIP_DesligadoComZero(t *testing.T) {
	s, _ := relogio(t, 0, false)
	if s.taxa != nil {
		t.Fatal("porMinuto 0 deveria desligar o limitador")
	}
	h := s.Handler()
	for i := range 50 {
		if rec := pedir(h, "/v1/capacidades", "203.0.113.7:1111", nil); rec.Code != http.StatusOK {
			t.Fatalf("com o limite desligado a requisição %d = %d", i, rec.Code)
		}
	}
}

// A sonda chega do mesmo endereço a cada poucos segundos. Limitá-la derrubaria
// o container são, que é o mesmo motivo de ela não exigir token.
func TestLimitePorIP_SondaNaoGastaFicha(t *testing.T) {
	s, _ := relogio(t, 1, false)
	h := s.Handler()
	for _, rota := range []string{"/healthz", "/healthz", "/readyz", "/readyz", "/healthz"} {
		if rec := pedir(h, rota, "10.0.0.1:1111", nil); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("%s tomou 429", rota)
		}
	}
	// E o orçamento do endereço continua intacto para o tráfego de verdade.
	if rec := pedir(h, "/v1/capacidades", "10.0.0.1:1111", nil); rec.Code != http.StatusOK {
		t.Errorf("a rota de verdade = %d, quero 200: a sonda não deveria ter gasto nada", rec.Code)
	}
}

// O limite vale ANTES do Bearer. Se só valesse depois, tentar tokens sairia ao
// ritmo da rede, e é justamente o token que protege o certificado de terceiros.
func TestLimitePorIP_ValeAntesDaAutenticacao(t *testing.T) {
	s, _ := relogio(t, 2, false)
	h := s.Handler()

	tentar := func() int {
		r := httptest.NewRequest(http.MethodGet, "/v1/capacidades", nil)
		r.RemoteAddr = "203.0.113.9:1234"
		r.Header.Set("Authorization", "Bearer chute")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		return rec.Code
	}
	if c := tentar(); c != http.StatusUnauthorized {
		t.Fatalf("primeira tentativa = %d, quero 401", c)
	}
	tentar()
	if c := tentar(); c != http.StatusTooManyRequests {
		t.Errorf("depois do teto = %d, quero 429: adivinhar token tem de esbarrar no limite", c)
	}
}

// --- endereço do chamador ---------------------------------------------------

func TestIPDoCliente(t *testing.T) {
	casos := []struct {
		nome         string
		confia       bool
		remoto       string
		xff, xreal   string
		quero, razao string
	}{
		{
			nome: "peer externo ignora o cabeçalho", confia: true,
			remoto: "203.0.113.7:1111", xff: "1.2.3.4",
			quero: "203.0.113.7",
			razao: "conexão direta: o cabeçalho é do atacante, e cada valor novo zeraria o limite",
		},
		{
			nome: "proxy interno vale", confia: true,
			remoto: "10.0.0.5:1111", xff: "198.51.100.9",
			quero: "198.51.100.9",
		},
		{
			nome: "cadeia: vale o mais à direita que é externo", confia: true,
			remoto: "10.0.0.5:1111", xff: "9.9.9.9, 198.51.100.9, 10.0.0.5",
			quero: "198.51.100.9",
			razao: "o 9.9.9.9 da esquerda pode ter sido inventado pelo chamador",
		},
		{
			nome: "cadeia toda interna vale a origem", confia: true,
			remoto: "10.0.0.5:1111", xff: "192.168.1.10, 10.0.0.5",
			quero: "192.168.1.10",
		},
		{
			nome: "desconfiar do proxy ignora tudo", confia: false,
			remoto: "10.0.0.5:1111", xff: "198.51.100.9",
			quero: "10.0.0.5",
		},
		{
			nome: "X-Real-IP quando não há cadeia", confia: true,
			remoto: "127.0.0.1:1111", xreal: "198.51.100.9",
			quero: "198.51.100.9",
		},
		{
			nome: "cabeçalho ilegível cai no peer", confia: true,
			remoto: "10.0.0.5:1111", xff: "não é um endereço",
			quero: "10.0.0.5",
		},
		{
			nome: "endereço com porta no cabeçalho", confia: true,
			remoto: "10.0.0.5:1111", xff: "198.51.100.9:5555",
			quero: "198.51.100.9",
		},
		{
			nome: "IPv6", confia: true,
			remoto: "[::1]:1111", xff: "2001:db8::1",
			quero: "2001:db8::1",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cfg := cfgTeste()
			cfg.TrustProxyHeaders = c.confia
			s := Novo(cfg, &acbr.Servicos{})

			r := httptest.NewRequest(http.MethodGet, "/v1/capacidades", nil)
			r.RemoteAddr = c.remoto
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if c.xreal != "" {
				r.Header.Set("X-Real-IP", c.xreal)
			}
			if got := s.ipDoCliente(r); got != c.quero {
				t.Errorf("ipDoCliente = %q, quero %q\n%s", got, c.quero, c.razao)
			}
		})
	}
}

// Um proxy pode mandar o cabeçalho repetido em vez de concatenado.
func TestClienteNaCadeia_CabecalhoRepetido(t *testing.T) {
	if got := clienteNaCadeia([]string{"9.9.9.9", "198.51.100.9, 10.0.0.5"}); got != "198.51.100.9" {
		t.Errorf("got %q", got)
	}
	if got := clienteNaCadeia(nil); got != "" {
		t.Errorf("cadeia vazia = %q, quero vazio", got)
	}
}

// Um XFF forjado por requisição não pode multiplicar o orçamento do atacante.
func TestLimitePorIP_XFFForjadoNaoEscapa(t *testing.T) {
	s, _ := relogio(t, 2, true)
	h := s.Handler()

	codigos := make([]int, 0, 4)
	for i := range 4 {
		rec := pedir(h, "/v1/capacidades", "203.0.113.7:1111",
			map[string]string{"X-Forwarded-For": fmt.Sprintf("1.2.3.%d", i)})
		codigos = append(codigos, rec.Code)
	}
	if codigos[3] != http.StatusTooManyRequests {
		t.Errorf("códigos %v: o cabeçalho forjado numa conexão direta não deveria zerar o saldo", codigos)
	}
}

// --- teto de memória --------------------------------------------------------

// O mapa é indexado por endereço, ou seja, por dado que o atacante escolhe.
// Sem teto, acompanhar viraria o vetor de exaustão que o limite deveria conter.
func TestLimitador_TetoDeClientesNaoCresceSemFim(t *testing.T) {
	l := novoLimitador(60)
	agora := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	l.agora = func() time.Time { return agora }

	for i := range maxClientes + 500 {
		l.permitir(fmt.Sprintf("198.51.%d.%d", i/256, i%256))
	}
	if len(l.clientes) > maxClientes {
		t.Errorf("acompanhando %d endereços, o teto é %d", len(l.clientes), maxClientes)
	}

	// Quem já estava sendo acompanhado continua limitado: o teto suspende o
	// limite para endereços NOVOS, não para os conhecidos.
	conhecido := "198.51.0.0"
	for range 60 {
		l.permitir(conhecido)
	}
	if ok, _ := l.permitir(conhecido); ok {
		t.Error("endereço já acompanhado deveria continuar esbarrando no teto")
	}

	// Passado um minuto todos repuseram tudo: balde cheio não guarda
	// informação, e a limpeza devolve o espaço.
	agora = agora.Add(2 * time.Minute)
	l.permitir("203.0.113.1")
	if len(l.clientes) > 10 {
		t.Errorf("depois da reposição sobraram %d baldes cheios", len(l.clientes))
	}
}

// Sob concorrência o saldo tem de ser exato: um balde contado errado deixa
// passar mais do que o configurado, e é sempre sob carga que isso acontece.
func TestLimitador_ContaCertoSobConcorrencia(t *testing.T) {
	const teto = 100
	l := novoLimitador(teto)
	agora := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	l.agora = func() time.Time { return agora } // relógio parado: nada repõe

	var mu sync.Mutex
	passaram := 0
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if ok, _ := l.permitir("203.0.113.7"); ok {
					mu.Lock()
					passaram++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	if passaram != teto {
		t.Errorf("passaram %d de 800 tentativas, o teto é %d", passaram, teto)
	}
}
