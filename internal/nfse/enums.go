package nfse

// Opcao é um par valor/rótulo para enums apresentados na UI.
type Opcao struct {
	Valor  int
	Rotulo string
}

// Enums do Padrão Nacional (rótulos extraídos da documentação dos XSD oficiais
// tiposComplexos_v1.01.xsd). Usados para apresentar selects amigáveis no lugar
// dos códigos numéricos.
var (
	// OpSimpNac: situação perante o Simples Nacional.
	OpSimpNac = []Opcao{
		{1, "Não optante"},
		{2, "Optante – MEI (Microempreendedor Individual)"},
		{3, "Optante – ME/EPP (Microempresa ou Empresa de Pequeno Porte)"},
	}
	// RegApTribSN: regime de apuração do SN (relevante quando opSimpNac = 3).
	RegApTribSN = []Opcao{
		{0, "Não se aplica"},
		{1, "Tributos federais e municipal pelo Simples Nacional"},
		{2, "Federais pelo SN; ISSQN por fora do SN (legislação municipal)"},
		{3, "Tributos federais e municipal por fora do SN"},
	}
	// RegEspTrib: regime especial de tributação.
	RegEspTrib = []Opcao{
		{0, "Nenhum"},
		{1, "Ato Cooperado (Cooperativa)"},
		{2, "Estimativa"},
		{3, "Microempresa Municipal"},
		{4, "Notário ou Registrador"},
		{5, "Profissional Autônomo"},
		{6, "Sociedade de Profissionais"},
		{9, "Outros"},
	}
)
