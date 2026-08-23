// Package boleto modela o boleto bancário (não-fiscal) e traduz o JSON para o
// INI combinado consumido pela ACBrLibBoleto ([Banco]/[Conta]/[Cedente]/
// [TituloN]). A geração é AGNÓSTICA ao banco: o banco é só config ([Banco]).
package boleto

import "log/slog"

// Pedido é o corpo de geração de boleto(s): a conta de cobrança (cedente +
// conta + banco) e a lista de títulos.
type Pedido struct {
	Conta   Conta    `json:"conta"`
	Titulos []Titulo `json:"titulos"`
}

// SemCredenciais devolve uma cópia do Pedido SEM o bloco de credenciais de WS
// (Conta.WS: ClientSecret, CertKEY/CertCRT).
//
// No projeto de origem ela sanitizava o registro de auditoria antes de gravar.
// Aqui não há persistência, então o risco mudou de lugar: o que sobra é ecoar ou
// logar o pedido. Use esta cópia em qualquer ponto que devolva o Pedido ao
// cliente. Receiver por valor: a cópia é sanitizada, o original fica intacto.
func (p Pedido) SemCredenciais() Pedido {
	p.Conta.WS = nil
	return p
}

// Conta reúne o banco, a conta corrente e o cedente: quem cobra e por onde.
type Conta struct {
	Banco string `json:"banco"` // número do banco: 001 = BB, 237 = Bradesco, 341 = Itaú
	// TipoCobranca identifica o banco E o layout de arquivo. Não é o número do
	// banco: uma instituição que emite no layout de outra tem valor próprio
	// aqui. Confirme com o seu banco qual layout foi contratado.
	TipoCobranca  int    `json:"tipoCobranca,omitempty" enum:"TipoCobranca"`
	CNAB          string `json:"CNAB,omitempty" enum:"CNAB"`
	Agencia       string `json:"agencia"` // número da agência, sem o dígito
	DigitoAgencia string `json:"digitoAgencia,omitempty"`
	NumeroConta   string `json:"conta"` // número da conta corrente, sem o dígito
	DigitoConta   string `json:"digitoConta,omitempty"`
	// Nome do cedente, quem recebe o pagamento.
	Nome           string `json:"nome"`
	Fantasia       string `json:"fantasia,omitempty"`
	CNPJCPF        string `json:"CNPJCPF"`
	TipoInscricao  int    `json:"tipoInscricao,omitempty" enum:"TipoInscricao"` // 1=CNPJ, 2=CPF
	TipoPessoa     int    `json:"tipoPessoa,omitempty"`
	Logradouro     string `json:"logradouro,omitempty"`
	Numero         string `json:"numero,omitempty"`
	Bairro         string `json:"bairro,omitempty"`
	Cidade         string `json:"cidade,omitempty"`
	CEP            string `json:"CEP,omitempty"`
	Complemento    string `json:"complemento,omitempty"`
	UF             string `json:"UF,omitempty"`
	Telefone       string `json:"telefone,omitempty"`
	CodigoCedente  string `json:"codigoCedente,omitempty"`
	Modalidade     string `json:"modalidade,omitempty"`
	Convenio       string `json:"convenio,omitempty"`
	CodTransmissao string `json:"codTransmissao,omitempty"`
	TipoCarteira   int    `json:"tipoCarteira,omitempty" enum:"TipoCarteira"` // 0=simples,1=registrada,2=eletronica
	TipoDocumento  int    `json:"tipoDocumento,omitempty"`
	RespEmis       int    `json:"respEmis,omitempty"`
	LayoutBol      int    `json:"layoutBol,omitempty"`
	// PIX (boleto híbrido).
	PixTipoChave int    `json:"pixTipoChave,omitempty"`
	PixChave     string `json:"pixChave,omitempty"`
	// WS: credenciais para registro online. Opcional.
	WS *ContaWS `json:"ws,omitempty"`
}

// ContaWS são as credenciais de integração online com a API do banco: OAuth
// (ClientID/Secret/Scope) + certificado mTLS (CRT/KEY em base64).
//
// Como o certificado A1 dos documentos fiscais, elas viajam no payload e não são
// persistidas. Os métodos String/LogValue abaixo existem para que um %v ou um
// slog distraído não as despeje em log: num serviço sem estado, é a única forma
// realista de um segredo do cliente escapar do processo.
type ContaWS struct {
	Ambiente     int    `json:"ambiente,omitempty" enum:"AmbienteWS"` // 1=produção, 2=homologação
	ClientID     string `json:"clientID,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	KeyUser      string `json:"keyUser,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IndicadorPix string `json:"indicadorPix,omitempty"`
	CertCRT      string `json:"certCRT,omitempty"` // base64 do .crt (mTLS)
	CertKEY      string `json:"certKEY,omitempty"` // base64 do .key (mTLS)
}

// String redige o conteúdo.
func (c ContaWS) String() string { return "ContaWS{redigido}" }

// LogValue redige no slog (que não usa String() para structs).
func (c ContaWS) LogValue() slog.Value { return slog.StringValue("ContaWS{redigido}") }

// Titulo é um boleto a ser cobrado. Um pedido pode trazer vários.
type Titulo struct {
	NumeroDocumento string   `json:"numeroDocumento"`
	SeuNumero       string   `json:"seuNumero,omitempty"`
	NossoNumero     string   `json:"nossoNumero,omitempty"`
	Carteira        string   `json:"carteira,omitempty"`
	Especie         string   `json:"especie,omitempty"`
	ValorDocumento  float64  `json:"valorDocumento"`
	Vencimento      string   `json:"vencimento"`
	DataDocumento   string   `json:"dataDocumento,omitempty"`
	Aceite          string   `json:"aceite,omitempty"`
	Sacado          Sacado   `json:"sacado"`
	Mensagem        string   `json:"mensagem,omitempty"`
	Instrucoes      []string `json:"instrucoes,omitempty"` // até 3 (Instrucao1-3)
	// Encargos.
	ValorMoraJuros  float64 `json:"valorMoraJuros,omitempty"`
	CodigoMora      string  `json:"codigoMora,omitempty"`
	PercentualMulta float64 `json:"percentualMulta,omitempty"`
	MultaValorFixo  float64 `json:"multaValorFixo,omitempty"`
	ValorDesconto   float64 `json:"valorDesconto,omitempty"`
	DataDesconto    string  `json:"dataDesconto,omitempty"`
	ValorAbatimento float64 `json:"valorAbatimento,omitempty"`
	// Vínculo fiscal.
	ChaveNFe string `json:"chaveNFe,omitempty"`
}

// Sacado é o pagador do boleto.
type Sacado struct {
	Nome        string `json:"nome"`
	CNPJCPF     string `json:"CNPJCPF"`
	Logradouro  string `json:"logradouro,omitempty"`
	Numero      string `json:"numero,omitempty"`
	Bairro      string `json:"bairro,omitempty"`
	Cidade      string `json:"cidade,omitempty"`
	UF          string `json:"UF,omitempty"`
	CEP         string `json:"CEP,omitempty"`
	Complemento string `json:"complemento,omitempty"`
	Email       string `json:"email,omitempty"`
}
