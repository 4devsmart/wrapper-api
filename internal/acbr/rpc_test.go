package acbr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// servicoFake embute indisponivel (que já satisfaz as quatro interfaces) e
// sobrescreve apenas o que o teste exercita.
type servicoFake struct {
	indisponivel

	iniRecebido    string
	sessaoRecebida Sessao
	result         Result
	err            error

	atraso     time.Duration
	emCurso    atomic.Int32
	pico       atomic.Int32
	chamadas   atomic.Int32
	versaoFake string
}

// handlerDeTeste é o RPCHandler sem reciclagem.
func handlerDeTeste(s *Servicos, slots int) http.Handler {
	h, _ := RPCHandler(s, slots, 0)
	return h
}

func (f *servicoFake) Emitir(t Sessao, ini string) (Result, error) {
	f.chamadas.Add(1)
	if n := f.emCurso.Add(1); n > f.pico.Load() {
		f.pico.Store(n)
	}
	defer f.emCurso.Add(-1)
	f.iniRecebido, f.sessaoRecebida = ini, t
	if f.atraso > 0 {
		time.Sleep(f.atraso)
	}
	return f.result, f.err
}

func (f *servicoFake) Version() (string, error) { return f.versaoFake, nil }

// servicosFake monta um *Servicos com o mesmo fake nos quatro serviços.
func servicosFake(f *servicoFake) *Servicos {
	return &Servicos{NFSe: f, CTe: f, MDFe: f, Boleto: f}
}

// clienteDe monta o binding remoto apontando para um endereço de worker.
func clienteDe(t *testing.T, endereco string, slots int, timeout time.Duration) *Servicos {
	t.Helper()
	return newRemotos(config.ACBr{
		Workers:       []string{endereco},
		WorkerSlots:   slots,
		WorkerTimeout: timeout,
	})
}

// workerDeTeste sobe o handler RPC num servidor HTTP efêmero.
func workerDeTeste(t *testing.T, f *servicoFake, slots int) string {
	t.Helper()
	ts := httptest.NewServer(handlerDeTeste(servicosFake(f), slots))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestRPC_RoundTripEmissao(t *testing.T) {
	f := &servicoFake{result: Result{
		Codigo:   0,
		Resposta: "situacao=100",
		XML:      "<NFSe/>",
		PDF:      []byte("%PDF-1.4 fake"),
	}}
	svc := clienteDe(t, workerDeTeste(t, f, 1), 1, 5*time.Second)

	sessao := Sessao{
		CNPJ:      "12345678000199",
		PFXBase64: "cGZ4LWZha2U=",
		SenhaPFX:  "segredo",
		Config:    []ConfigKV{{Section: "NFSe", Key: "CodigoMunicipio", Value: "3550308"}},
	}
	res, err := svc.NFSe.Emitir(sessao, "[DPS]\nnDPS=1\n")
	if err != nil {
		t.Fatalf("Emitir devolveu erro: %v", err)
	}

	if res.Resposta != "situacao=100" || res.XML != "<NFSe/>" {
		t.Errorf("Result não voltou íntegro: %+v", res)
	}
	if string(res.PDF) != "%PDF-1.4 fake" {
		t.Errorf("PDF não sobreviveu ao round-trip: %q", res.PDF)
	}
	if f.iniRecebido != "[DPS]\nnDPS=1\n" {
		t.Errorf("INI chegou alterado no worker: %q", f.iniRecebido)
	}
	// O certificado e as chaves de config precisam atravessar intactos, senão a
	// emissão falha do outro lado por motivo obscuro.
	if f.sessaoRecebida.PFXBase64 != sessao.PFXBase64 || f.sessaoRecebida.SenhaPFX != sessao.SenhaPFX {
		t.Errorf("Sessao chegou incompleto: %+v", f.sessaoRecebida)
	}
	if len(f.sessaoRecebida.Config) != 1 || f.sessaoRecebida.Config[0].Value != "3550308" {
		t.Errorf("ConfigKV não atravessou: %+v", f.sessaoRecebida.Config)
	}
}

func TestRPC_PreservaErrUnavailable(t *testing.T) {
	// O stub responde ErrUnavailable a tudo: é o cenário "worker sem a lib".
	svc := clienteDe(t, workerDeTeste(t, &servicoFake{err: ErrUnavailable}, 1), 1, 5*time.Second)

	_, err := svc.NFSe.Emitir(Sessao{}, "[DPS]")
	if err == nil {
		t.Fatal("esperava erro")
	}
	// A sentinela precisa sobreviver ao RPC: é ela que faz o handler responder
	// 503 lib_indisponivel em vez de 502.
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("ErrUnavailable não sobreviveu ao RPC: %v", err)
	}
}

func TestRPC_ErroDaLibNaoViraIndisponivel(t *testing.T) {
	f := &servicoFake{
		result: Result{Codigo: -3, Resposta: "rejeição 214: nDPS duplicado"},
		err:    errors.New("falha na emissão"),
	}
	svc := clienteDe(t, workerDeTeste(t, f, 1), 1, 5*time.Second)

	res, err := svc.NFSe.Emitir(Sessao{}, "[DPS]")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if errors.Is(err, ErrUnavailable) {
		t.Error("falha da lib não pode virar 'indisponível' (mudaria 502 em 503)")
	}
	// Result e erro coexistem: os handlers usam o código/resposta da lib mesmo
	// quando a chamada falhou.
	if res.Codigo != -3 || !strings.Contains(res.Resposta, "nDPS duplicado") {
		t.Errorf("Result não acompanhou o erro: %+v", res)
	}
}

func TestRemoto_WorkerInacessivelEhIndisponivel(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "nao-existe.sock")
	svc := clienteDe(t, socket, 1, 2*time.Second)

	_, err := svc.NFSe.Emitir(Sessao{}, "[DPS]")
	if err == nil {
		t.Fatal("esperava erro")
	}
	// Falhou ao CONECTAR: a chamada nunca saiu, logo repetir é seguro → 503.
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("worker fora do ar deveria ser ErrUnavailable, veio: %v", err)
	}
}

func TestRemoto_QuedaDuranteChamadaAvisaRiscoDeDuplicidade(t *testing.T) {
	// Simula o worker morrendo no meio da chamada (o caso do SIGSEGV): a conexão
	// é aceita e derrubada sem resposta.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack falhou: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer ts.Close()

	svc := clienteDe(t, ts.URL, 1, 5*time.Second)
	_, err := svc.NFSe.Emitir(Sessao{}, "[DPS]")
	if err == nil {
		t.Fatal("esperava erro")
	}
	// Aqui o documento PODE ter sido transmitido antes do crash. Marcar como
	// "indisponível" (503) convidaria o cliente a repetir e duplicar a nota.
	if errors.Is(err, ErrUnavailable) {
		t.Error("queda no meio da chamada não pode ser ErrUnavailable")
	}
	// A sentinela é o que o handler usa para responder 504 resultado_indeterminado
	//: texto solto não é acionável por quem consome a API.
	if !errors.Is(err, ErrIndeterminado) {
		t.Errorf("queda em voo deveria ser ErrIndeterminado, veio: %v", err)
	}
}

func TestRPC_MetodoDesconhecido(t *testing.T) {
	f := &servicoFake{}
	ts := httptest.NewServer(handlerDeTeste(servicosFake(f), 1))
	defer ts.Close()

	resp, err := http.Post(ts.URL+RotaRPC, "application/json",
		strings.NewReader(`{"servico":"nfse","metodo":"NaoExiste"}`))
	if err != nil {
		t.Fatalf("post falhou: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("método desconhecido deveria dar 400, veio %d", resp.StatusCode)
	}
}

func TestPool_LimitaChamadasSimultaneas(t *testing.T) {
	// Com 1 vaga, duas emissões concorrentes precisam se enfileirar: é o que
	// garante que um crash da lib leve junto UMA requisição, não várias.
	f := &servicoFake{atraso: 60 * time.Millisecond}
	svc := clienteDe(t, workerDeTeste(t, f, 1), 1, 5*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.CTe.Emitir(Sessao{}, "[infCTe]"); err != nil {
				t.Errorf("Emitir concorrente falhou: %v", err)
			}
		}()
	}
	wg.Wait()

	if f.chamadas.Load() != 4 {
		t.Errorf("esperava 4 chamadas, houve %d", f.chamadas.Load())
	}
	if pico := f.pico.Load(); pico != 1 {
		t.Errorf("com 1 vaga o pico de chamadas nativas simultâneas deve ser 1, foi %d", pico)
	}
}

func TestRemoto_BackendEVersao(t *testing.T) {
	svc := clienteDe(t, workerDeTeste(t, &servicoFake{versaoFake: "2.1.0.301"}, 1), 1, 5*time.Second)

	if b := svc.NFSe.Backend(); b != "worker" {
		t.Errorf("Backend do binding remoto deveria ser 'worker', veio %q", b)
	}
	v, err := svc.MDFe.Version()
	if err != nil {
		t.Fatalf("Version falhou: %v", err)
	}
	if v != "2.1.0.301" {
		t.Errorf("Version não atravessou o RPC: %q", v)
	}
}

// TestPool_WorkerMortoNaoRoubaAFila trava a quarentena. Sem ela, o worker caído
// devolve a vaga em microssegundos, é reescolhido imediatamente e absorve a fila
// inteira servindo erro: enquanto o worker vivo, mais lento, atende uma só.
func TestPool_WorkerMortoNaoRoubaAFila(t *testing.T) {
	f := &servicoFake{}
	vivo := workerDeTeste(t, f, 1)
	morto := filepath.Join(t.TempDir(), "morto.sock") // socket inexistente

	svc := newRemotos(config.ACBr{
		Workers:       []string{morto, vivo},
		WorkerSlots:   1,
		WorkerTimeout: 5 * time.Second,
	})

	// Sequência de chamadas: a primeira pode cair no morto; as seguintes têm de
	// encontrar o vivo, porque o morto fica de quarentena.
	var indisponiveis int
	for i := 0; i < 6; i++ {
		if _, err := svc.NFSe.Emitir(Sessao{}, "[DPS]"); err != nil {
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("erro inesperado: %v", err)
			}
			indisponiveis++
		}
	}
	if indisponiveis > 1 {
		t.Errorf("o worker morto deveria sair do rodízio após a 1ª falha, mas falhou %d de 6", indisponiveis)
	}
	if f.chamadas.Load() < 5 {
		t.Errorf("o worker vivo deveria ter atendido o resto, atendeu %d", f.chamadas.Load())
	}
}

// Cada chamada leva mais da metade do prazo: com fila e chamada no mesmo relógio,
// a segunda transmitiria e estouraria no meio, virando desfecho indeterminado.
func TestPool_EsperaNaFilaNaoEncurtaAChamada(t *testing.T) {
	f := &servicoFake{atraso: 300 * time.Millisecond}
	svc := clienteDe(t, workerDeTeste(t, f, 1), 1, 500*time.Millisecond)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.CTe.Emitir(Sessao{}, "[infCTe]"); err != nil {
				t.Errorf("Emitir falhou por ter esperado na fila: %v", err)
			}
		}()
	}
	wg.Wait()
}

// Desistir na fila é seguro: nada saiu, e o cliente precisa saber que pode repetir.
func TestPool_FilaEsgotadaNaoViraIndeterminado(t *testing.T) {
	f := &servicoFake{}
	p := novoPool(config.ACBr{
		Workers:       []string{workerDeTeste(t, f, 1)},
		WorkerSlots:   1,
		WorkerTimeout: 50 * time.Millisecond,
	})
	<-p.livres // a única vaga fica ocupada até o fim do teste

	_, err := p.chamar(Pedido{Servico: ServicoCTe, Metodo: "Emitir"})
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrIndeterminado) {
		t.Errorf("quero ErrUnavailable, veio: %v", err)
	}
	if n := f.chamadas.Load(); n != 0 {
		t.Errorf("%d chamadas chegaram à lib; a da fila não podia sair", n)
	}
}

// Com 1 vaga e uma emissão em curso, o /readyz tem de responder que está pronto.
func TestRemoto_SaudavelNaoDisputaAVaga(t *testing.T) {
	f := &servicoFake{atraso: 300 * time.Millisecond}
	svc := clienteDe(t, workerDeTeste(t, f, 1), 1, 5*time.Second)
	h := svc.NFSe.(interface{ Saudavel(context.Context) error })

	emitindo := make(chan struct{})
	go func() {
		defer close(emitindo)
		_, _ = svc.CTe.Emitir(Sessao{}, "[infCTe]")
	}()
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := h.Saudavel(ctx); err != nil {
		t.Errorf("worker ocupado deveria estar pronto: %v", err)
	}
	<-emitindo
}

// TestRemoto_SaudavelRefleteOWorker: é o sinal que alimenta o /readyz, sem ele
// a API se declara pronta com a emissão morta.
func TestRemoto_SaudavelRefleteOWorker(t *testing.T) {
	svc := clienteDe(t, workerDeTeste(t, &servicoFake{}, 1), 1, 5*time.Second)
	h, ok := svc.NFSe.(interface{ Saudavel(context.Context) error })
	if !ok {
		t.Fatal("o binding remoto deveria expor Saudavel (o /readyz depende disso)")
	}
	if err := h.Saudavel(context.Background()); err != nil {
		t.Errorf("worker no ar deveria ser saudável: %v", err)
	}

	semWorker := newRemotos(config.ACBr{
		Workers:       []string{filepath.Join(t.TempDir(), "nao-existe.sock")},
		WorkerSlots:   1,
		WorkerTimeout: 2 * time.Second,
	})
	h2 := semWorker.NFSe.(interface{ Saudavel(context.Context) error })
	if err := h2.Saudavel(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("sem worker, Saudavel deveria acusar indisponibilidade, veio: %v", err)
	}
}

// TestReciclagemSinalizaAposTeto: o worker precisa avisar que atingiu o teto
// para o processo drenar e sair: é o que impede corrupção de uma chamada de
// atravessar para as próximas indefinidamente.
func TestReciclagemSinalizaAposTeto(t *testing.T) {
	f := &servicoFake{}
	h, reciclar := RPCHandler(servicosFake(f), 1, 2)
	ts := httptest.NewServer(h)
	defer ts.Close()
	svc := clienteDe(t, ts.URL, 1, 5*time.Second)

	if _, err := svc.NFSe.Emitir(Sessao{}, "[DPS]"); err != nil {
		t.Fatalf("1ª chamada: %v", err)
	}
	select {
	case <-reciclar:
		t.Fatal("não deveria reciclar antes do teto")
	default:
	}

	if _, err := svc.NFSe.Emitir(Sessao{}, "[DPS]"); err != nil {
		t.Fatalf("2ª chamada: %v", err)
	}
	select {
	case <-reciclar:
	case <-time.After(time.Second):
		t.Error("atingido o teto, o worker deveria sinalizar a reciclagem")
	}
}

// TestReciclagemDesligadaPorPadrao: com max_calls=0 o worker nunca recicla.
// reciclar com um único worker abriria janela de indisponibilidade.
func TestReciclagemDesligadaPorPadrao(t *testing.T) {
	f := &servicoFake{}
	h, reciclar := RPCHandler(servicosFake(f), 1, 0)
	ts := httptest.NewServer(h)
	defer ts.Close()
	svc := clienteDe(t, ts.URL, 1, 5*time.Second)

	for i := 0; i < 5; i++ {
		if _, err := svc.CTe.Emitir(Sessao{}, "[infCTe]"); err != nil {
			t.Fatalf("chamada %d: %v", i, err)
		}
	}
	select {
	case <-reciclar:
		t.Error("com max_calls=0 não deveria reciclar")
	default:
	}
}

// TestDespachoCobreTodasAsInterfaces é a rede para o passo que o compilador NÃO
// pega: adicionar método na interface + cgo + stub + cliente e esquecer o case
// no despacho. O compilador aceita, e o worker responde "método desconhecido"
// só em runtime. Aqui a lista de métodos vem por reflexão, então qualquer
// método novo entra no teste sozinho.
func TestDespachoCobreTodasAsInterfaces(t *testing.T) {
	// Backend/Close são locais ao processo: não trafegam pelo RPC.
	locais := map[string]bool{"Backend": true, "Close": true}

	casos := map[string]reflect.Type{
		ServicoNFSe:   reflect.TypeOf((*NFSeServico)(nil)).Elem(),
		ServicoCTe:    reflect.TypeOf((*CTeServico)(nil)).Elem(),
		ServicoMDFe:   reflect.TypeOf((*MDFeServico)(nil)).Elem(),
		ServicoBoleto: reflect.TypeOf((*BoletoServico)(nil)).Elem(),
	}
	svc := servicosFake(&servicoFake{})
	for servico, tipo := range casos {
		for i := 0; i < tipo.NumMethod(); i++ {
			metodo := tipo.Method(i).Name
			if locais[metodo] {
				continue
			}
			if _, conhecido := Despachar(svc, Pedido{Servico: servico, Metodo: metodo}); !conhecido {
				t.Errorf("%s/%s: método da interface sem case no despacho RPC (rpcserver.go)", servico, metodo)
			}
		}
	}
}

// --- contrato de gerar e transmitir -------------------------------------------------

// fluxoFake registra o que cada chamada recebeu, para provar que o certificado só
// aparece na transmissão.
type fluxoFake struct {
	indisponivel

	montarINI     string
	montarSessao  Sessao
	transmitirXML string
	transmitirTn  Sessao
	validarINI    string

	resMontar     Result
	resTransmitir Result
	errValidar    error
}

func (f *fluxoFake) MontarXML(t Sessao, ini string) (Result, error) {
	f.montarINI, f.montarSessao = ini, t
	return f.resMontar, nil
}

func (f *fluxoFake) Transmitir(t Sessao, xml string) (Result, error) {
	f.transmitirXML, f.transmitirTn = xml, t
	return f.resTransmitir, nil
}

func (f *fluxoFake) ValidarRegras(_ Sessao, ini string) (Result, error) {
	f.validarINI = ini
	return Result{}, f.errValidar
}

func TestRPC_GerarETransmitirAtravessamOWorker(t *testing.T) {
	f := &fluxoFake{
		resMontar:     Result{Codigo: 0, XML: "<CTe><infCte Id=\"CTe35...\"/></CTe>"},
		resTransmitir: Result{Codigo: 0, Resposta: "cStat=100", XML: "<cteProc/>"},
	}
	ts := httptest.NewServer(handlerDeTeste(&Servicos{NFSe: f, CTe: f, MDFe: f, Boleto: f}, 1))
	t.Cleanup(ts.Close)
	cli := clienteDe(t, ts.URL, 1, 5*time.Second).CTe

	// Gerar: monta e valida SEM certificado. É o que permite testar a camada de
	// montagem: o ativo real do projeto, sem um .pfx no repositório.
	res, err := cli.MontarXML(Sessao{CNPJ: "12345678000199"}, "[infCTe]\nversao=4.00")
	if err != nil {
		t.Fatalf("MontarXML: %v", err)
	}
	if res.XML != f.resMontar.XML {
		t.Errorf("XML da geração = %q, quero %q", res.XML, f.resMontar.XML)
	}
	if f.montarSessao.PFXBase64 != "" {
		t.Errorf("a geração recebeu certificado (%q): ela existe justamente para não precisar de um", f.montarSessao.PFXBase64)
	}

	// Transmitir: envia o XML gerado, agora com o certificado na sessão.
	res, err = cli.Transmitir(Sessao{CNPJ: "12345678000199", PFXBase64: "cGZ4", SenhaPFX: "s3nh4"}, f.resMontar.XML)
	if err != nil {
		t.Fatalf("Transmitir: %v", err)
	}
	if f.transmitirXML != f.resMontar.XML {
		t.Errorf("a transmissão recebeu %q; deveria receber exatamente o XML gerado", f.transmitirXML)
	}
	if f.transmitirTn.PFXBase64 != "cGZ4" || f.transmitirTn.SenhaPFX != "s3nh4" {
		t.Errorf("certificado não chegou à transmissão: %+v", f.transmitirTn)
	}
	if res.XML != "<cteProc/>" || res.Resposta != "cStat=100" {
		t.Errorf("resposta da transmissão incompleta: %+v", res)
	}
}

// A NFS-e não expõe validação de regras de negócio. O cliente precisa conseguir
// distinguir isso de "a lib caiu": um é 422 (não existe), o outro é 503 (tente
// depois). A sentinela tem de sobreviver ao JSON do RPC.
func TestRPC_NaoSuportadoNaoViraIndisponivel(t *testing.T) {
	f := &fluxoFake{errValidar: ErrNaoSuportado}
	ts := httptest.NewServer(handlerDeTeste(&Servicos{NFSe: f, CTe: f, MDFe: f, Boleto: f}, 1))
	t.Cleanup(ts.Close)

	_, err := clienteDe(t, ts.URL, 1, 5*time.Second).NFSe.ValidarRegras(Sessao{}, "[DPS]")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !errors.Is(err, ErrNaoSuportado) {
		t.Errorf("sentinela ErrNaoSuportado não sobreviveu ao RPC: %v", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Errorf("não suportado virou indisponível, viraria 503 no lugar de 422: %v", err)
	}
}

// Transmitir é o ÚNICO ponto em que um documento sai para a SEFAZ. Se o worker
// morre no meio, o desfecho é desconhecido: pode ter sido autorizado. O erro
// precisa ser ErrIndeterminado (nunca ErrUnavailable, que autoriza repetir).
func TestTransmitir_QuedaNoMeioNaoAutorizaRepetir(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, RotaRPC) {
			ts.CloseClientConnections() // simula o SIGSEGV no meio da chamada
		}
	}))
	t.Cleanup(ts.Close)

	_, err := clienteDe(t, ts.URL, 1, 5*time.Second).CTe.Transmitir(Sessao{}, "<CTe/>")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !errors.Is(err, ErrIndeterminado) {
		t.Errorf("queda durante a transmissão deveria ser ErrIndeterminado, veio: %v", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Errorf("queda durante a transmissão marcada como indisponível, convida a repetir e duplicar: %v", err)
	}
	if !strings.Contains(err.Error(), "Transmitir") {
		t.Errorf("a mensagem deveria nomear a operação em risco: %v", err)
	}
}

// TestQuedaEmMetodoLocalNaoViraIndeterminado cobre a outra metade do conserto da
// validação: agora que uma falha ao GERAR chega ao cliente em vez de virar
// "validação ok", ela precisa chegar com o status certo.
//
// MontarXML e ValidarRegras não abrem conexão com a SEFAZ. Se o worker cai no
// meio de uma delas, não há desfecho a descobrir: repetir é seguro. Marcar como
// indeterminado mandaria o cliente "consultar pela chave" de um documento que
// nunca saiu, e cuja chave ele ainda nem tem.
func TestQuedaEmMetodoLocalNaoViraIndeterminado(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, RotaRPC) {
			ts.CloseClientConnections() // o SIGSEGV no meio da chamada
		}
	}))
	t.Cleanup(ts.Close)

	svc := clienteDe(t, ts.URL, 1, 5*time.Second)
	for nome, chamada := range map[string]func() error{
		"MontarXML":     func() error { _, err := svc.CTe.MontarXML(Sessao{}, "[ide]"); return err },
		"ValidarRegras": func() error { _, err := svc.CTe.ValidarRegras(Sessao{}, "[ide]"); return err },
		"RenderizarPDF": func() error { _, err := svc.CTe.RenderizarPDF(Sessao{}, "<CTe/>"); return err },
	} {
		t.Run(nome, func(t *testing.T) {
			err := chamada()
			if err == nil {
				t.Fatal("esperava erro")
			}
			if errors.Is(err, ErrIndeterminado) {
				t.Errorf("%s não transmite nada; indeterminado manda consultar uma chave que não existe: %v", nome, err)
			}
			if !errors.Is(err, ErrUnavailable) {
				t.Errorf("%s deveria ser ErrUnavailable (503, repetir é seguro), veio: %v", nome, err)
			}
		})
	}
}

// O nome do campo no fio é "tenant", mesmo com o tipo chamado Sessao. O protocolo
// não tem versão e API e worker sobem separados: trocar a tag faria o lado antigo
// ignorar o campo sem erro nenhum e abrir a sessão sem certificado nem município.
func TestPedido_CampoDaSessaoNoFioContinuaTenant(t *testing.T) {
	b, err := json.Marshal(Pedido{Servico: ServicoNFSe, Metodo: "Emitir", Sessao: Sessao{CNPJ: "123"}})
	if err != nil {
		t.Fatal(err)
	}
	var cru map[string]json.RawMessage
	if err := json.Unmarshal(b, &cru); err != nil {
		t.Fatal(err)
	}
	if _, ok := cru["tenant"]; !ok {
		t.Fatalf("o campo tem de continuar \"tenant\" no JSON: %s", b)
	}
	var volta Pedido
	if err := json.Unmarshal([]byte(`{"tenant":{"CNPJ":"456"}}`), &volta); err != nil {
		t.Fatal(err)
	}
	if volta.Sessao.CNPJ != "456" {
		t.Errorf("um pedido da versão anterior perdeu a sessão: %+v", volta)
	}
}
