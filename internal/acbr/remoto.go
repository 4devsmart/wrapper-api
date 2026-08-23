package acbr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// remoto implementa as cinco interfaces de serviço delegando a um processo
// fiscal-worker (ver rpc.go). Um único tipo atende todas elas, como o stub
// indisponivel, o que muda é o campo servico, que endereça o binding do outro
// lado.
type remoto struct {
	servico string
	pool    *poolWorkers
}

// remoto satisfaz as cinco interfaces de serviço.
var (
	_ NFSeServico   = remoto{}
	_ CTeServico    = remoto{}
	_ MDFeServico   = remoto{}
	_ NFeServico    = remoto{}
	_ BoletoServico = remoto{}
)

// newRemotos cria os bindings que delegam ao(s) fiscal-worker(s).
func newRemotos(cfg config.ACBr) *Servicos {
	p := novoPool(cfg)
	return &Servicos{
		NFSe:   remoto{servico: ServicoNFSe, pool: p},
		CTe:    remoto{servico: ServicoCTe, pool: p},
		MDFe:   remoto{servico: ServicoMDFe, pool: p},
		NFe:    remoto{servico: ServicoNFe, pool: p},
		Boleto: remoto{servico: ServicoBoleto, pool: p},
	}
}

// --- pool de workers --------------------------------------------------------

// poolWorkers distribui as chamadas entre os workers configurados. O canal de
// destinos livres é, ao mesmo tempo, o roteador e o semáforo de concorrência: a
// capacidade é len(workers)*slots, e é ela que limita quantas chamadas nativas
// correm em paralelo: limite que NÃO existia com a lib no processo da API.
type poolWorkers struct {
	livres  chan *destino
	timeout time.Duration
	// quarentena é quanto tempo um worker que falhou o DIAL fica fora do rodízio.
	// Sem isso ele devolve a vaga na hora, é reescolhido em microssegundos e passa
	// a absorver a fila inteira servindo erro: enquanto um worker vivo está
	// ocupado. Vale só para falha de conexão (worker morto/reiniciando).
	quarentena time.Duration
}

type destino struct {
	rotulo string
	url    string
	cli    *http.Client
}

func novoPool(cfg config.ACBr) *poolWorkers {
	slots := cfg.WorkerSlots
	if slots <= 0 {
		slots = 1
	}
	timeout := cfg.WorkerTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	p := &poolWorkers{
		livres:     make(chan *destino, len(cfg.Workers)*slots),
		timeout:    timeout,
		quarentena: 3 * time.Second,
	}
	for _, addr := range cfg.Workers {
		d := novoDestino(addr)
		for i := 0; i < slots; i++ {
			p.livres <- d // o mesmo destino ocupa `slots` vagas
		}
	}
	return p
}

// novoDestino monta o cliente HTTP de um endereço de worker. Aceita caminho de
// socket unix ("/run/wrapper/fiscal.sock", "unix:///run/..."), usado em produção, e
// URL http:// (útil em teste).
func novoDestino(addr string) *destino {
	tr := &http.Transport{
		// SEM keep-alive, de propósito: com conexões reaproveitadas o
		// net/http reenvia automaticamente o pedido quando encontra uma
		// conexão ociosa que morreu, e um reenvio silencioso de Emitir é
		// como se duplica um documento fiscal. Uma conexão nova por chamada
		// custa microssegundos num socket unix.
		DisableKeepAlives: true,
	}

	socket := strings.TrimPrefix(addr, "unix://")
	if strings.HasPrefix(socket, "/") {
		tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		}
		// O host da URL é irrelevante no socket unix, mas precisa ser válido.
		return &destino{rotulo: socket, url: "http://worker" + RotaRPC, cli: &http.Client{Transport: tr}}
	}
	return &destino{rotulo: addr, url: strings.TrimSuffix(addr, "/") + RotaRPC, cli: &http.Client{Transport: tr}}
}

// saudavel tenta o /healthz de um worker livre. Não consome vaga por muito
// tempo nem toca na lib nativa.
func (p *poolWorkers) saudavel(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var d *destino
	select {
	case d = <-p.livres:
		defer func() { p.livres <- d }()
	case <-ctx.Done():
		return fmt.Errorf("nenhum worker fiscal livre: %w", ErrUnavailable)
	}

	url := strings.TrimSuffix(d.url, RotaRPC) + RotaHealth
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := d.cli.Do(req)
	if err != nil {
		return fmt.Errorf("worker fiscal %s inacessível: %w", d.rotulo, ErrUnavailable)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("worker fiscal %s respondeu HTTP %d: %w", d.rotulo, res.StatusCode, ErrUnavailable)
	}
	return nil
}

// chamar adquire um worker livre, faz a chamada e devolve a vaga.
func (p *poolWorkers) chamar(pedido Pedido) (Resposta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	var d *destino
	var mortoNoDial bool
	select {
	case d = <-p.livres:
		defer func() {
			if mortoNoDial {
				// Devolve depois da quarentena, sem segurar o chamador.
				go func(d *destino) {
					time.Sleep(p.quarentena)
					p.livres <- d
				}(d)
				return
			}
			p.livres <- d
		}()
	case <-ctx.Done():
		return Resposta{}, fmt.Errorf("nenhum worker fiscal livre em %s: %w", p.timeout, ErrUnavailable)
	}

	corpo, err := json.Marshal(pedido)
	if err != nil {
		return Resposta{}, fmt.Errorf("serializar pedido: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(corpo))
	if err != nil {
		return Resposta{}, fmt.Errorf("montar pedido: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := d.cli.Do(req)
	if err != nil {
		var falha error
		falha, mortoNoDial = erroDeTransporte(d, pedido, err)
		return Resposta{}, falha
	}
	defer func() { _ = res.Body.Close() }()

	var resp Resposta
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return Resposta{}, fmt.Errorf("resposta ilegível do worker %s (HTTP %d): %w", d.rotulo, res.StatusCode, err)
	}
	if res.StatusCode != http.StatusOK && resp.Erro == "" {
		resp.Erro = fmt.Sprintf("worker devolveu HTTP %d", res.StatusCode)
	}
	if resp.Erro != "" {
		return resp, erroDaResposta(resp)
	}
	return resp, nil
}

// erroDaResposta reconstrói, do lado do cliente, a sentinela que o worker
// marcou. É o inverso de marcarErro (rpcserver.go): sem isto o errors.Is do
// chamador nunca casaria, porque a sentinela não atravessa JSON.
func erroDaResposta(resp Resposta) error {
	switch {
	case resp.Indisponivel:
		return fmt.Errorf("%s: %w", resp.Erro, ErrUnavailable)
	case resp.NaoSuportado:
		return fmt.Errorf("%s: %w", resp.Erro, ErrNaoSuportado)
	default:
		return errors.New(resp.Erro)
	}
}

// erroDeTransporte classifica a falha de comunicação com o worker e diz se ela
// foi ao DISCAR (o que põe o destino de quarentena). A distinção importa e é
// fiscal, não cosmética:
//
//   - falha ao CONECTAR → a chamada nunca saiu; é ErrUnavailable (503), e repetir
//     é seguro;
//   - queda ou prazo estourado DEPOIS de enviada, num método que TRANSMITE → o
//     worker pode ter crashado já com o documento na SEFAZ. Nunca é
//     ErrUnavailable (vira 502), e a mensagem avisa que repetir pode duplicar;
//   - queda depois de enviada num método LOCAL (gerar, validar, renderizar) →
//     nada saiu do host, então não há desfecho a descobrir: é ErrUnavailable de
//     novo, e repetir é seguro. Ver metodosLocais em rpc.go.
func erroDeTransporte(d *destino, p Pedido, err error) (falha error, falhouNoDial bool) {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return fmt.Errorf("worker fiscal %s inacessível (%v): %w", d.rotulo, err, ErrUnavailable), true
	}
	if metodosLocais[p.Metodo] {
		return fmt.Errorf("worker fiscal %s caiu durante %s/%s (%v): a operação não transmite nada, repetir é seguro: %w",
			d.rotulo, p.Servico, p.Metodo, err, ErrUnavailable), false
	}
	return fmt.Errorf("worker fiscal %s falhou durante %s/%s (%v): confira antes de repetir: %w",
		d.rotulo, p.Servico, p.Metodo, err, ErrIndeterminado), false
}

// --- implementação das interfaces -------------------------------------------

// rpc executa um método no worker.
func (r remoto) rpc(metodo string, t TenantConfig, a Args) (Resposta, error) {
	return r.pool.chamar(Pedido{Servico: r.servico, Metodo: metodo, Tenant: t, Args: a})
}

// chamar é o atalho para os métodos que devolvem (Result, error).
func (r remoto) chamar(metodo string, t TenantConfig, a Args) (Result, error) {
	resp, err := r.rpc(metodo, t, a)
	return resp.Result(), err
}

func (remoto) Backend() string { return "worker" }

// Saudavel confirma que existe worker atendendo. Usa o /healthz do worker (não
// uma chamada nativa): responde a pergunta "dá para emitir?" sem custo de sessão
// e sem depender de a lib estar carregada. Alimenta o /readyz da API.
func (r remoto) Saudavel(ctx context.Context) error { return r.pool.saudavel(ctx) }
func (remoto) Close() error                         { return nil } // a conexão é por chamada; nada a liberar

func (r remoto) Version() (string, error) {
	resp, err := r.rpc("Version", TenantConfig{}, Args{})
	return resp.Versao, err
}

// Servico (comum a NFS-e/CT-e/MDF-e).

func (r remoto) Emitir(t TenantConfig, ini string) (Result, error) {
	return r.chamar("Emitir", t, Args{INI: ini})
}

func (r remoto) MontarXML(t TenantConfig, ini string) (Result, error) {
	return r.chamar("MontarXML", t, Args{INI: ini})
}

func (r remoto) ValidarRegras(t TenantConfig, ini string) (Result, error) {
	return r.chamar("ValidarRegras", t, Args{INI: ini})
}

func (r remoto) Transmitir(t TenantConfig, xml string) (Result, error) {
	return r.chamar("Transmitir", t, Args{XML: xml})
}

func (r remoto) Consultar(t TenantConfig, chave string) (Result, error) {
	return r.chamar("Consultar", t, Args{Chave: chave})
}

func (r remoto) Cancelar(t TenantConfig, ini string) (Result, error) {
	return r.chamar("Cancelar", t, Args{INI: ini})
}

func (r remoto) ObterPDF(t TenantConfig, chave string) (Result, error) {
	return r.chamar("ObterPDF", t, Args{Chave: chave})
}

func (r remoto) RenderizarPDF(t TenantConfig, xml string) (Result, error) {
	return r.chamar("RenderizarPDF", t, Args{XML: xml})
}

func (r remoto) XmlParaIni(t TenantConfig, xml string) (Result, error) {
	return r.chamar("XmlParaIni", t, Args{XML: xml})
}

// NFS-e.

func (r remoto) SubstituirNFSe(t TenantConfig, iniNovaDPS string, sub SubstituicaoNFSe) (Result, error) {
	return r.chamar("SubstituirNFSe", t, Args{INI: iniNovaDPS, Sub: &sub})
}

func (r remoto) ConsultarDFe(t TenantConfig, nsu int) (Result, error) {
	return r.chamar("ConsultarDFe", t, Args{NSU: nsu})
}

func (r remoto) ConsultarDPSPorChave(t TenantConfig, chaveDPS string) (Result, error) {
	return r.chamar("ConsultarDPSPorChave", t, Args{Chave: chaveDPS})
}

func (r remoto) ConsultarPorNumero(t TenantConfig, numero string, pagina int) (Result, error) {
	return r.chamar("ConsultarPorNumero", t, Args{Numero: numero, Pagina: pagina})
}

func (r remoto) ConsultarPorFaixa(t TenantConfig, inicial, final string, pagina int) (Result, error) {
	return r.chamar("ConsultarPorFaixa", t, Args{Numero: inicial, NumeroFinal: final, Pagina: pagina})
}

func (r remoto) ConsultarPorRps(t TenantConfig, numeroRps, serie, tipo, codVerificacao string) (Result, error) {
	return r.chamar("ConsultarPorRps", t, Args{
		Numero: numeroRps, Serie: serie, Tipo: tipo, CodVerificacao: codVerificacao,
	})
}

func (r remoto) ConsultarSituacao(t TenantConfig, protocolo, numLote string) (Result, error) {
	return r.chamar("ConsultarSituacao", t, Args{Protocolo: protocolo, NumLote: numLote})
}

func (r remoto) ConsultarLoteRps(t TenantConfig, protocolo, numLote string) (Result, error) {
	return r.chamar("ConsultarLoteRps", t, Args{Protocolo: protocolo, NumLote: numLote})
}

// CT-e / MDF-e.

func (r remoto) CartaCorrecao(t TenantConfig, ini string) (Result, error) {
	return r.chamar("CartaCorrecao", t, Args{INI: ini})
}

func (r remoto) Encerrar(t TenantConfig, ini string) (Result, error) {
	return r.chamar("Encerrar", t, Args{INI: ini})
}

func (r remoto) EnviarEvento(t TenantConfig, ini string) (Result, error) {
	return r.chamar("EnviarEvento", t, Args{INI: ini})
}

func (r remoto) SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error) {
	return r.chamar("SalvarEventoPDF", t, Args{XMLDoc: xmlDoc, XMLEvento: xmlEvento})
}

func (r remoto) DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error) {
	return r.chamar("DistribuicaoDFe", t, Args{Dist: &p})
}

func (r remoto) StatusServico(t TenantConfig) (Result, error) {
	return r.chamar("StatusServico", t, Args{})
}

func (r remoto) ConsultarRecibo(t TenantConfig, recibo string) (Result, error) {
	return r.chamar("ConsultarRecibo", t, Args{Recibo: recibo})
}

func (r remoto) ConsultaCadastro(t TenantConfig, uf, nDocumento string, ehIE bool) (Result, error) {
	return r.chamar("ConsultaCadastro", t, Args{UF: uf, Documento: nDocumento, EhIE: ehIE})
}

func (r remoto) ConsultaNaoEncerrados(t TenantConfig, cnpj string) (Result, error) {
	return r.chamar("ConsultaNaoEncerrados", t, Args{CNPJ: cnpj})
}

// NF-e (distribuição + manifestação).

func (r remoto) Manifestar(t TenantConfig, ini string) (Result, error) {
	return r.chamar("Manifestar", t, Args{INI: ini})
}

// Boleto.

func (r remoto) GerarPDF(t TenantConfig, ini string) (Result, error) {
	return r.chamar("GerarPDF", t, Args{INI: ini})
}

func (r remoto) GerarRemessa(t TenantConfig, ini string, numArquivo int) (Result, error) {
	return r.chamar("GerarRemessa", t, Args{INI: ini, NumArquivo: numArquivo})
}

func (r remoto) LerRetorno(t TenantConfig, configINI, retornoConteudo string) (Result, error) {
	return r.chamar("LerRetorno", t, Args{ConfigINI: configINI, Retorno: retornoConteudo})
}

func (r remoto) Registrar(t TenantConfig, op BoletoOnline) (Result, error) {
	return r.chamar("Registrar", t, Args{Boleto: &op})
}

func (r remoto) ConsultarTitulos(t TenantConfig, op BoletoOnline) (Result, error) {
	return r.chamar("ConsultarTitulos", t, Args{Boleto: &op})
}
