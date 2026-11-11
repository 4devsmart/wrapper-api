package acbr

import (
	"encoding/json"
	"errors"
	"github.com/4devsmart/wrapper-api/internal/platform/httpx"
	"net/http"
	"sync"
)

// RPCHandler expõe os serviços do binding local como um servidor RPC (ver
// rpc.go). É Go puro: funciona sobre qualquer implementação das interfaces,
// então dá para testá-lo sem a lib nativa.
//
// slots limita quantas chamadas nativas correm em paralelo neste processo. É
// defesa em profundidade: o cliente já respeita o mesmo limite, mas o worker não
// depende disso para se proteger. slots<=0 vira 1.
//
// O canal devolvido é fechado quando o worker atinge maxCalls, sinalizando ao
// processo que ele deve drenar e sair (o supervisor sobe um novo). maxCalls<=0
// desliga a reciclagem.
func RPCHandler(s *Servicos, slots, maxCalls int) (http.Handler, <-chan struct{}) {
	if slots <= 0 {
		slots = 1
	}
	vagas := make(chan struct{}, slots)
	reciclar := make(chan struct{})
	var (
		mu        sync.Mutex
		atendidas int
	)
	contar := func() {
		if maxCalls <= 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		atendidas++
		if atendidas == maxCalls {
			close(reciclar)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+RotaHealth, func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST "+RotaRPC, func(w http.ResponseWriter, r *http.Request) {
		var p Pedido
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			httpx.JSON(w, http.StatusBadRequest, Resposta{Erro: "pedido inválido: " + err.Error()})
			return
		}

		// Aguarda uma vaga; desiste se o chamador cancelar (timeout do cliente).
		select {
		case vagas <- struct{}{}:
			defer func() { <-vagas }()
		case <-r.Context().Done():
			httpx.JSON(w, http.StatusServiceUnavailable, Resposta{
				Erro: "worker ocupado: nenhuma vaga liberou a tempo", Indisponivel: true,
			})
			return
		}

		contar()
		resp, conhecido := Despachar(s, p)
		if !conhecido {
			httpx.JSON(w, http.StatusBadRequest, Resposta{
				Erro: "método desconhecido: " + p.Servico + "/" + p.Metodo,
			})
			return
		}
		httpx.JSON(w, http.StatusOK, resp)
	})
	return mux, reciclar
}

// Despachar executa o método pedido no serviço correspondente. conhecido=false
// quando o par serviço/método não existe: erro de protocolo, não de operação.
func Despachar(s *Servicos, p Pedido) (resp Resposta, conhecido bool) {
	switch p.Servico {
	case ServicoNFSe:
		if r, ok := chamarComum(s.NFSe, p); ok {
			return r, true
		}
		return chamarNFSe(s.NFSe, p)
	case ServicoCTe:
		if r, ok := chamarComum(s.CTe, p); ok {
			return r, true
		}
		return chamarCTe(s.CTe, p)
	case ServicoMDFe:
		if r, ok := chamarComum(s.MDFe, p); ok {
			return r, true
		}
		return chamarMDFe(s.MDFe, p)
	case ServicoBoleto:
		return chamarBoleto(s.Boleto, p)
	}
	return Resposta{}, false
}

// chamarComum trata os métodos da interface Servico, compartilhados por
// NFS-e/CT-e/MDF-e.
func chamarComum(sv Servico, p Pedido) (Resposta, bool) {
	a := p.Args
	switch p.Metodo {
	case "Version":
		return respostaVersao(sv.Version()), true
	case "Emitir":
		return resposta(sv.Emitir(p.Sessao, a.INI)), true
	case "MontarXML":
		return resposta(sv.MontarXML(p.Sessao, a.INI)), true
	case "ValidarRegras":
		return resposta(sv.ValidarRegras(p.Sessao, a.INI)), true
	case "Transmitir":
		return resposta(sv.Transmitir(p.Sessao, a.XML)), true
	case "Consultar":
		return resposta(sv.Consultar(p.Sessao, a.Chave)), true
	case "Cancelar":
		return resposta(sv.Cancelar(p.Sessao, a.INI)), true
	case "ObterPDF":
		return resposta(sv.ObterPDF(p.Sessao, a.Chave)), true
	case "RenderizarPDF":
		return resposta(sv.RenderizarPDF(p.Sessao, a.XML)), true
	}
	return Resposta{}, false
}

func chamarNFSe(sv NFSeServico, p Pedido) (Resposta, bool) {
	a := p.Args
	switch p.Metodo {
	case "SubstituirNFSe":
		var sub SubstituicaoNFSe
		if a.Sub != nil {
			sub = *a.Sub
		}
		return resposta(sv.SubstituirNFSe(p.Sessao, a.INI, sub)), true
	case "EnviarEvento":
		return resposta(sv.EnviarEvento(p.Sessao, a.INI)), true
	case "InformacoesProvedor":
		return resposta(sv.InformacoesProvedor(p.Sessao)), true
	case "ConsultarDFe":
		return resposta(sv.ConsultarDFe(p.Sessao, a.CNPJ, a.NSU)), true
	case "ConsultarDPSPorChave":
		return resposta(sv.ConsultarDPSPorChave(p.Sessao, a.Chave)), true
	case "ConsultarPorNumero":
		return resposta(sv.ConsultarPorNumero(p.Sessao, a.Numero, a.Pagina)), true
	case "ConsultarPorFaixa":
		return resposta(sv.ConsultarPorFaixa(p.Sessao, a.Numero, a.NumeroFinal, a.Pagina)), true
	case "ConsultarPorRps":
		return resposta(sv.ConsultarPorRps(p.Sessao, a.Numero, a.Serie, a.Tipo, a.CodVerificacao)), true
	case "ConsultarSituacao":
		return resposta(sv.ConsultarSituacao(p.Sessao, a.Protocolo, a.NumLote)), true
	case "ConsultarLoteRps":
		return resposta(sv.ConsultarLoteRps(p.Sessao, a.Protocolo, a.NumLote)), true
	}
	return Resposta{}, false
}

func chamarCTe(sv CTeServico, p Pedido) (Resposta, bool) {
	a := p.Args
	switch p.Metodo {
	case "CartaCorrecao":
		return resposta(sv.CartaCorrecao(p.Sessao, a.INI)), true
	case "EnviarEvento":
		return resposta(sv.EnviarEvento(p.Sessao, a.INI)), true
	case "StatusServico":
		return resposta(sv.StatusServico(p.Sessao)), true
	case "ConsultaCadastro":
		return resposta(sv.ConsultaCadastro(p.Sessao, a.UF, a.Documento, a.EhIE)), true
	case "SalvarEventoPDF":
		return resposta(sv.SalvarEventoPDF(p.Sessao, a.XMLDoc, a.XMLEvento)), true
	}
	return Resposta{}, false
}

func chamarMDFe(sv MDFeServico, p Pedido) (Resposta, bool) {
	a := p.Args
	switch p.Metodo {
	case "Encerrar":
		return resposta(sv.Encerrar(p.Sessao, a.INI)), true
	case "EnviarEvento":
		return resposta(sv.EnviarEvento(p.Sessao, a.INI)), true
	case "StatusServico":
		return resposta(sv.StatusServico(p.Sessao)), true
	case "ConsultaNaoEncerrados":
		return resposta(sv.ConsultaNaoEncerrados(p.Sessao, a.CNPJ)), true
	case "SalvarEventoPDF":
		return resposta(sv.SalvarEventoPDF(p.Sessao, a.XMLDoc, a.XMLEvento)), true
	}
	return Resposta{}, false
}

func chamarBoleto(sv BoletoServico, p Pedido) (Resposta, bool) {
	a := p.Args
	switch p.Metodo {
	case "Version":
		return respostaVersao(sv.Version()), true
	case "GerarPDF":
		return resposta(sv.GerarPDF(p.Sessao, a.INI)), true
	case "GerarRemessa":
		return resposta(sv.GerarRemessa(p.Sessao, a.INI, a.NumArquivo)), true
	case "LerRetorno":
		return resposta(sv.LerRetorno(p.Sessao, a.ConfigINI, a.Retorno)), true
	case "Registrar":
		return resposta(sv.Registrar(p.Sessao, boletoOnline(a))), true
	}
	return Resposta{}, false
}

func boletoOnline(a Args) BoletoOnline {
	if a.Boleto == nil {
		return BoletoOnline{}
	}
	return *a.Boleto
}

// resposta traduz o par (Result, error) do binding para o protocolo, preservando
// a sentinela ErrUnavailable.
func resposta(r Result, err error) Resposta {
	resp := Resposta{Codigo: r.Codigo, Resposta: r.Resposta, XML: r.XML, PDF: r.PDF}
	marcarErro(&resp, err)
	return resp
}

func respostaVersao(v string, err error) Resposta {
	resp := Resposta{Versao: v}
	marcarErro(&resp, err)
	return resp
}

// marcarErro registra a mensagem e as sentinelas que precisam sobreviver à
// serialização. Um errors.Is não atravessa JSON, sem estes booleanos o cliente
// receberia só texto e teria de adivinhar o status HTTP pela mensagem.
func marcarErro(resp *Resposta, err error) {
	if err == nil {
		return
	}
	resp.Erro = err.Error()
	resp.Indisponivel = errors.Is(err, ErrUnavailable)
	resp.NaoSuportado = errors.Is(err, ErrNaoSuportado)
}
