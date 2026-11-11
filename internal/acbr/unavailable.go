package acbr

// indisponivel é uma implementação de MDFeServico (e, portanto, de Servico) que
// reporta a lib nativa indisponível. Usada pelos stubs (build sem -tags acbrlib)
// e como placeholder dos serviços ainda não ligados no binding cgo.
type indisponivel struct {
	nome string // rótulo do serviço (nfse/cte/mdfe): informativo
}

// indisponivel satisfaz as quatro interfaces de serviço (é o stub sem -tags
// acbrlib, compilado no CI). No build cgo, a conformidade dos tipos reais é
// garantida pela construção em newServicos (acbr_cgo.go).
var (
	_ NFSeServico   = indisponivel{}
	_ CTeServico    = indisponivel{}
	_ MDFeServico   = indisponivel{}
	_ BoletoServico = indisponivel{}
)

func (indisponivel) Backend() string                          { return "stub" }
func (indisponivel) Version() (string, error)                 { return "", ErrUnavailable }
func (indisponivel) Emitir(Sessao, string) (Result, error)    { return Result{}, ErrUnavailable }
func (indisponivel) MontarXML(Sessao, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) ValidarRegras(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Transmitir(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Consultar(Sessao, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) Cancelar(Sessao, string) (Result, error)  { return Result{}, ErrUnavailable }
func (indisponivel) ObterPDF(Sessao, string) (Result, error)  { return Result{}, ErrUnavailable }
func (indisponivel) RenderizarPDF(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Encerrar(Sessao, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) EnviarEvento(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) CartaCorrecao(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) SalvarEventoPDF(Sessao, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) SubstituirNFSe(Sessao, string, SubstituicaoNFSe) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) InformacoesProvedor(Sessao) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarDFe(Sessao, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) StatusServico(Sessao) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) ConsultaCadastro(Sessao, string, string, bool) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultaNaoEncerrados(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarDPSPorChave(Sessao, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorNumero(Sessao, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorFaixa(Sessao, string, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorRps(Sessao, string, string, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarSituacao(Sessao, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarLoteRps(Sessao, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) GerarPDF(Sessao, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) GerarRemessa(Sessao, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) LerRetorno(Sessao, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Registrar(Sessao, BoletoOnline) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Close() error { return nil }
