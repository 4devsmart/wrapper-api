package acbr

// indisponivel é uma implementação de MDFeServico (e, portanto, de Servico) que
// reporta a lib nativa indisponível. Usada pelos stubs (build sem -tags acbrlib)
// e como placeholder dos serviços ainda não ligados no binding cgo.
type indisponivel struct {
	nome string // rótulo do serviço (nfse/cte/mdfe) — informativo
}

// indisponivel satisfaz as cinco interfaces de serviço (é o stub sem -tags
// acbrlib, compilado no CI). No build cgo, a conformidade dos tipos reais é
// garantida pela construção em newServicos (acbr_cgo.go).
var (
	_ NFSeServico   = indisponivel{}
	_ CTeServico    = indisponivel{}
	_ MDFeServico   = indisponivel{}
	_ NFeServico    = indisponivel{}
	_ BoletoServico = indisponivel{}
)

func (indisponivel) Backend() string                                { return "stub" }
func (indisponivel) Version() (string, error)                       { return "", ErrUnavailable }
func (indisponivel) Emitir(TenantConfig, string) (Result, error)    { return Result{}, ErrUnavailable }
func (indisponivel) MontarXML(TenantConfig, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) ValidarRegras(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Transmitir(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Consultar(TenantConfig, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) Cancelar(TenantConfig, string) (Result, error)  { return Result{}, ErrUnavailable }
func (indisponivel) ObterPDF(TenantConfig, string) (Result, error)  { return Result{}, ErrUnavailable }
func (indisponivel) RenderizarPDF(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) XmlParaIni(TenantConfig, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) Encerrar(TenantConfig, string) (Result, error)   { return Result{}, ErrUnavailable }
func (indisponivel) EnviarEvento(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) CartaCorrecao(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) SalvarEventoPDF(TenantConfig, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) DistribuicaoDFe(TenantConfig, DistDFeParams) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) SubstituirNFSe(TenantConfig, string, SubstituicaoNFSe) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarDFe(TenantConfig, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Manifestar(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) StatusServico(TenantConfig) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) ConsultarRecibo(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultaCadastro(TenantConfig, string, string, bool) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultaNaoEncerrados(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarDPSPorChave(TenantConfig, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorNumero(TenantConfig, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorFaixa(TenantConfig, string, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarPorRps(TenantConfig, string, string, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarSituacao(TenantConfig, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarLoteRps(TenantConfig, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) GerarPDF(TenantConfig, string) (Result, error) { return Result{}, ErrUnavailable }
func (indisponivel) GerarRemessa(TenantConfig, string, int) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) LerRetorno(TenantConfig, string, string) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Registrar(TenantConfig, BoletoOnline) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) ConsultarTitulos(TenantConfig, BoletoOnline) (Result, error) {
	return Result{}, ErrUnavailable
}
func (indisponivel) Close() error { return nil }
