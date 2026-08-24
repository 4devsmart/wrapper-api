package acbr

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// O despacho RPC é uma tradução escrita à mão, duas vezes: o cliente monta
// Pedido{Metodo, Args} e o servidor lê aquele Args de volta para chamar o
// método. Nada no compilador liga as duas pontas: um nome de método com typo
// ou um campo de Args trocado compila, passa nos testes, e vira dado errado
// atravessando a fronteira em silêncio.
//
// O teste que existia (TestDespachoCobreTodasAsInterfaces) confere o SERVIDOR
// contra a interface: prova que existe um case para cada método. Não olha o
// cliente, e não olha os argumentos. Trocar Args{NumeroFinal: final} por
// Args{Numero: final} passa por ele inteiro.
//
// Este aqui percorre TODOS os métodos das cinco interfaces, chama cada um pelo
// cliente remoto com uma sentinela distinta por posição, e cobra do outro lado
// o mesmo nome e os mesmos valores, na mesma ordem.

// espiao implementa as cinco interfaces SEM embutir nada. É deliberado: método
// novo numa interface quebra a COMPILAÇÃO deste arquivo, e quem o acrescentar
// é obrigado a passar por aqui. Embutindo indisponivel, como faz o servicoFake,
// o método novo nasceria fora da conferência.
type espiao struct {
	metodo string
	tenant TenantConfig
	args   []any
}

func (e *espiao) reg(metodo string, t TenantConfig, args ...any) (Result, error) {
	e.metodo, e.tenant, e.args = metodo, t, args
	return Result{Resposta: "ok"}, nil
}

func (e *espiao) Backend() string { return "espiao" }
func (e *espiao) Close() error    { return nil }
func (e *espiao) Version() (string, error) {
	e.metodo, e.args = "Version", nil
	return "1.0", nil
}

// Servico (comum a NFS-e, CT-e e MDF-e).
func (e *espiao) Emitir(t TenantConfig, ini string) (Result, error) {
	return e.reg("Emitir", t, ini)
}
func (e *espiao) MontarXML(t TenantConfig, ini string) (Result, error) {
	return e.reg("MontarXML", t, ini)
}
func (e *espiao) ValidarRegras(t TenantConfig, ini string) (Result, error) {
	return e.reg("ValidarRegras", t, ini)
}
func (e *espiao) Transmitir(t TenantConfig, xml string) (Result, error) {
	return e.reg("Transmitir", t, xml)
}
func (e *espiao) Consultar(t TenantConfig, chave string) (Result, error) {
	return e.reg("Consultar", t, chave)
}
func (e *espiao) Cancelar(t TenantConfig, ini string) (Result, error) {
	return e.reg("Cancelar", t, ini)
}
func (e *espiao) ObterPDF(t TenantConfig, chave string) (Result, error) {
	return e.reg("ObterPDF", t, chave)
}
func (e *espiao) RenderizarPDF(t TenantConfig, xml string) (Result, error) {
	return e.reg("RenderizarPDF", t, xml)
}
func (e *espiao) XmlParaIni(t TenantConfig, xml string) (Result, error) {
	return e.reg("XmlParaIni", t, xml)
}

// NFS-e.
func (e *espiao) SubstituirNFSe(t TenantConfig, ini string, sub SubstituicaoNFSe) (Result, error) {
	return e.reg("SubstituirNFSe", t, ini, sub)
}
func (e *espiao) ConsultarDFe(t TenantConfig, nsu int) (Result, error) {
	return e.reg("ConsultarDFe", t, nsu)
}
func (e *espiao) ConsultarDPSPorChave(t TenantConfig, chave string) (Result, error) {
	return e.reg("ConsultarDPSPorChave", t, chave)
}
func (e *espiao) ConsultarPorNumero(t TenantConfig, numero string, pagina int) (Result, error) {
	return e.reg("ConsultarPorNumero", t, numero, pagina)
}
func (e *espiao) ConsultarPorFaixa(t TenantConfig, inicial, final string, pagina int) (Result, error) {
	return e.reg("ConsultarPorFaixa", t, inicial, final, pagina)
}
func (e *espiao) ConsultarPorRps(t TenantConfig, numero, serie, tipo, cod string) (Result, error) {
	return e.reg("ConsultarPorRps", t, numero, serie, tipo, cod)
}
func (e *espiao) ConsultarSituacao(t TenantConfig, protocolo, numLote string) (Result, error) {
	return e.reg("ConsultarSituacao", t, protocolo, numLote)
}
func (e *espiao) ConsultarLoteRps(t TenantConfig, protocolo, numLote string) (Result, error) {
	return e.reg("ConsultarLoteRps", t, protocolo, numLote)
}

// CT-e e MDF-e.
func (e *espiao) CartaCorrecao(t TenantConfig, ini string) (Result, error) {
	return e.reg("CartaCorrecao", t, ini)
}
func (e *espiao) Encerrar(t TenantConfig, ini string) (Result, error) {
	return e.reg("Encerrar", t, ini)
}
func (e *espiao) EnviarEvento(t TenantConfig, ini string) (Result, error) {
	return e.reg("EnviarEvento", t, ini)
}
func (e *espiao) DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error) {
	return e.reg("DistribuicaoDFe", t, p)
}
func (e *espiao) StatusServico(t TenantConfig) (Result, error) {
	return e.reg("StatusServico", t)
}
func (e *espiao) ConsultarRecibo(t TenantConfig, recibo string) (Result, error) {
	return e.reg("ConsultarRecibo", t, recibo)
}
func (e *espiao) ConsultaCadastro(t TenantConfig, uf, doc string, ehIE bool) (Result, error) {
	return e.reg("ConsultaCadastro", t, uf, doc, ehIE)
}
func (e *espiao) ConsultaNaoEncerrados(t TenantConfig, cnpj string) (Result, error) {
	return e.reg("ConsultaNaoEncerrados", t, cnpj)
}
func (e *espiao) SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error) {
	return e.reg("SalvarEventoPDF", t, xmlDoc, xmlEvento)
}

// NF-e.
func (e *espiao) Manifestar(t TenantConfig, ini string) (Result, error) {
	return e.reg("Manifestar", t, ini)
}

// Boleto.
func (e *espiao) GerarPDF(t TenantConfig, ini string) (Result, error) {
	return e.reg("GerarPDF", t, ini)
}
func (e *espiao) GerarRemessa(t TenantConfig, ini string, numArquivo int) (Result, error) {
	return e.reg("GerarRemessa", t, ini, numArquivo)
}
func (e *espiao) LerRetorno(t TenantConfig, configINI, retorno string) (Result, error) {
	return e.reg("LerRetorno", t, configINI, retorno)
}
func (e *espiao) Registrar(t TenantConfig, op BoletoOnline) (Result, error) {
	return e.reg("Registrar", t, op)
}
func (e *espiao) ConsultarTitulos(t TenantConfig, op BoletoOnline) (Result, error) {
	return e.reg("ConsultarTitulos", t, op)
}

// --- o teste ----------------------------------------------------------------

func TestDespacho_CadaMetodoChegaComOsArgumentosCertos(t *testing.T) {
	e := &espiao{}
	ts := httptest.NewServer(RPCHandler(&Servicos{NFSe: e, CTe: e, MDFe: e, NFe: e, Boleto: e}, 1))
	t.Cleanup(ts.Close)
	svc := newRemotos(config.ACBr{Workers: []string{ts.URL}, WorkerSlots: 1, WorkerTimeout: 5 * time.Second})

	tenant := TenantConfig{CNPJ: "12345678000199", PFXBase64: "cGZ4", SenhaPFX: "segredo"}

	casos := []struct {
		servico string
		cliente any
		iface   reflect.Type
	}{
		{ServicoNFSe, svc.NFSe, reflect.TypeOf((*NFSeServico)(nil)).Elem()},
		{ServicoCTe, svc.CTe, reflect.TypeOf((*CTeServico)(nil)).Elem()},
		{ServicoMDFe, svc.MDFe, reflect.TypeOf((*MDFeServico)(nil)).Elem()},
		{ServicoNFe, svc.NFe, reflect.TypeOf((*NFeServico)(nil)).Elem()},
		{ServicoBoleto, svc.Boleto, reflect.TypeOf((*BoletoServico)(nil)).Elem()},
	}

	conferidos := 0
	for _, c := range casos {
		v := reflect.ValueOf(c.cliente)
		for i := range c.iface.NumMethod() {
			m := c.iface.Method(i)
			switch m.Name {
			case "Backend", "Close", "Version":
				continue // não atravessam a fronteira com argumentos
			}
			t.Run(c.servico+"/"+m.Name, func(t *testing.T) {
				entradas, esperado := sentinelas(t, m.Type, tenant)
				*e = espiao{}

				saida := v.MethodByName(m.Name).Call(entradas)
				if err, _ := saida[len(saida)-1].Interface().(error); err != nil {
					t.Fatalf("a chamada não chegou ao worker: %v", err)
				}
				// Nome errado no cliente vira "método desconhecido" no servidor,
				// e o espião não registra nada.
				if e.metodo != m.Name {
					t.Fatalf("o worker executou %q; o cliente devia ter pedido %q", e.metodo, m.Name)
				}
				if e.tenant.PFXBase64 != tenant.PFXBase64 {
					t.Errorf("o tenant não atravessou: %+v", e.tenant)
				}
				if !reflect.DeepEqual(e.args, esperado) {
					t.Errorf("os argumentos chegaram trocados ou incompletos.\n  enviado:  %#v\n  recebido: %#v",
						esperado, e.args)
				}
			})
			conferidos++
		}
	}
	// Se a varredura parar de encontrar métodos, o teste passaria vazio.
	if conferidos < 30 {
		t.Errorf("só %d métodos conferidos; a varredura das interfaces quebrou", conferidos)
	}
}

// sentinelas monta os argumentos de uma chamada, um valor distinto por posição,
// e devolve também o que o espião deve receber do outro lado. Distinto por
// posição é o que faz um campo de Args trocado aparecer: dois argumentos do
// mesmo tipo com o mesmo valor esconderiam a troca.
func sentinelas(t *testing.T, tipo reflect.Type, tenant TenantConfig) ([]reflect.Value, []any) {
	t.Helper()
	var entradas []reflect.Value
	var esperado []any
	for i := range tipo.NumIn() {
		in := tipo.In(i)
		if in == reflect.TypeOf(TenantConfig{}) {
			entradas = append(entradas, reflect.ValueOf(tenant))
			continue
		}
		v := sentinela(t, in, i)
		entradas = append(entradas, v)
		esperado = append(esperado, v.Interface())
	}
	return entradas, esperado
}

func sentinela(t *testing.T, tipo reflect.Type, pos int) reflect.Value {
	t.Helper()
	switch tipo {
	case reflect.TypeOf(DistDFeParams{}):
		return reflect.ValueOf(DistDFeParams{
			UFAutor: 35, CNPJCPF: "12345678000199", Modo: DistDFeModo("ultnsu"),
			NSU: "000000000000042", Chave: "chave-da-distribuicao",
		})
	case reflect.TypeOf(SubstituicaoNFSe{}):
		return reflect.ValueOf(SubstituicaoNFSe{
			NumeroNFSe: "n-1", SerieNFSe: "s-2", CodigoCancelamento: "c-3",
			MotivoCancelamento: "m-4", NumeroLote: "l-5", CodigoVerificacao: "v-6",
		})
	case reflect.TypeOf(BoletoOnline{}):
		return reflect.ValueOf(BoletoOnline{
			INI: "ini-1", ConfigINI: "config-2", FiltroINI: "filtro-3",
			Operacao: 2, CertCRT: []byte("crt"), CertKEY: []byte("key"),
		})
	}
	switch tipo.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("arg-%d", pos)).Convert(tipo)
	case reflect.Int:
		return reflect.ValueOf(8100000 + pos)
	case reflect.Bool:
		return reflect.ValueOf(true)
	}
	t.Fatalf("sentinela não sabe montar %s: acrescente o caso", tipo)
	return reflect.Value{}
}

// metodosLocais é outra lista de STRINGS que precisa casar com nomes de método,
// e é ela que decide o status quando o worker cai no meio de uma chamada: 503
// para quem não transmitiu nada, 502 para quem pode ter transmitido.
//
// Um nome com typo ali não quebra nada visível: o método simplesmente deixa de
// ser reconhecido como local, e uma queda ao MONTAR o XML passa a mandar o
// cliente "consultar pela chave" de um documento que nunca saiu, e cuja chave
// ele justamente ainda não tem.
//
// O erro na outra direção (método que transmite listado como local) é o
// perigoso de verdade, e esse nenhum teste pega: é decisão humana, e está
// documentada no comentário da lista.
func TestMetodosLocais_TodosExistemNasInterfaces(t *testing.T) {
	daInterface := map[string]bool{}
	for _, tipo := range []reflect.Type{
		reflect.TypeOf((*NFSeServico)(nil)).Elem(),
		reflect.TypeOf((*CTeServico)(nil)).Elem(),
		reflect.TypeOf((*MDFeServico)(nil)).Elem(),
		reflect.TypeOf((*NFeServico)(nil)).Elem(),
		reflect.TypeOf((*BoletoServico)(nil)).Elem(),
	} {
		for i := range tipo.NumMethod() {
			daInterface[tipo.Method(i).Name] = true
		}
	}
	if len(daInterface) < 20 {
		t.Fatalf("só %d métodos encontrados; a varredura quebrou", len(daInterface))
	}

	for metodo := range metodosLocais {
		if !daInterface[metodo] {
			t.Errorf("metodosLocais cita %q, que não é método de interface nenhuma: "+
				"uma queda nesse método viraria 502 e mandaria consultar um documento que nunca saiu", metodo)
		}
	}
}
