package fiscal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- ambiente: um valor, dois códigos, nunca em desacordo -------------------

// TestAmbiente_OrdinalETpAmbConcordam cobre o segundo defeito: o ordinal da
// sessão e o tpAmb do XML saíam de funções diferentes, com normalizações
// diferentes. "Producao" abria a sessão em produção e escrevia tpAmb=2 no
// documento: rejeição 252 na melhor hipótese.
func TestAmbiente_OrdinalETpAmbConcordam(t *testing.T) {
	producao := []string{"producao", "Producao", "PRODUCAO", " producao ", "PrOdUcAo"}
	homologacao := []string{"", "homologacao", "Homologacao", "HOMOLOGACAO", " homologacao "}

	for _, amb := range producao {
		if o, tp := AmbienteOrdinal(amb), TpAmb(amb); o != "0" || tp != "1" {
			t.Errorf("%q: sessão=%s tpAmb=%s, esperava produção nos dois (0 e 1)", amb, o, tp)
		}
	}
	for _, amb := range homologacao {
		if o, tp := AmbienteOrdinal(amb), TpAmb(amb); o != "1" || tp != "2" {
			t.Errorf("%q: sessão=%s tpAmb=%s, esperava homologação nos dois (1 e 2)", amb, o, tp)
		}
	}
}

func TestNormalizarAmbiente_RecusaOQueNaoReconhece(t *testing.T) {
	for _, amb := range []string{"prod", "production", "hom", "producao1", "teste"} {
		if _, ok := NormalizarAmbiente(amb); ok {
			t.Errorf("%q foi aceito; valor não reconhecido tem de ser recusado, não virar homologação", amb)
		}
	}
	for _, amb := range []string{"", "producao", "homologacao"} {
		if _, ok := NormalizarAmbiente(amb); !ok {
			t.Errorf("%q deveria ser aceito", amb)
		}
	}
}

func TestAmbienteValido_RespondeErroDoCliente(t *testing.T) {
	w := httptest.NewRecorder()
	if _, ok := AmbienteValido(w, "prod"); ok {
		t.Fatal(`"prod" não pode passar`)
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, esperava 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ambiente_invalido") {
		t.Errorf("o código do erro precisa ser acionável: %s", w.Body.String())
	}

	w = httptest.NewRecorder()
	amb, ok := AmbienteValido(w, "  PRODUCAO ")
	if !ok || amb != AmbienteProducao {
		t.Errorf("esperava %q normalizado, veio %q (ok=%v)", AmbienteProducao, amb, ok)
	}
}

// AmbienteDoXML vivia copiado em três pacotes. É decisão de SEGURANÇA: é ela
// que impede um documento de homologação de ir para o webservice de produção,
// e não tinha teste em nenhuma das três cópias.
func TestAmbienteDoXML(t *testing.T) {
	casos := map[string]string{
		`<CTe><ide><tpAmb>1</tpAmb></ide></CTe>`: AmbienteProducao,
		`<CTe><ide><tpAmb>2</tpAmb></ide></CTe>`: AmbienteHomologacao,
		"<ide><tpAmb> 1 </tpAmb></ide>":          AmbienteProducao,
		"<ide><tpAmb>\n\t2\n</tpAmb></ide>":      AmbienteHomologacao,
		`<CTe><ide><cUF>35</cUF></ide></CTe>`:    "",
		`<CTe><ide><tpAmb>9</tpAmb></ide></CTe>`: "",
		``:                                       "",
	}
	for xml, quero := range casos {
		if got := AmbienteDoXML(xml); got != quero {
			t.Errorf("AmbienteDoXML(%q) = %q, quero %q", xml, got, quero)
		}
	}
}

// A transmissão usa Primeiro(AmbienteDoXML(xml), pedido): o XML manda, e o
// pedido só vale quando o XML não diz nada. Trocar a ordem faria o cliente
// mandar para produção um documento montado em homologação.
func TestAmbienteDoXML_TemPrecedenciaSobreOPedido(t *testing.T) {
	xml := `<CTe><ide><tpAmb>2</tpAmb></ide></CTe>`
	if got := Primeiro(AmbienteDoXML(xml), AmbienteProducao); got != AmbienteHomologacao {
		t.Errorf("o pedido venceu o XML: %q", got)
	}
	if got := Primeiro(AmbienteDoXML("<sem/>"), AmbienteProducao); got != AmbienteProducao {
		t.Errorf("sem tpAmb no XML deveria valer o pedido: %q", got)
	}
}

// AmbienteDoPedido e ModalSuportado vinham copiados nos módulos de documento,
// com o mesmo corpo e o nome do campo trocado na mensagem.
func TestAmbienteDoPedido(t *testing.T) {
	casos := []struct {
		nome     string
		ambiente string
		tpAmb    int
		campo    string
		quero    int    // 0 = passa
		normal   string // ambiente já normalizado
	}{
		{"normaliza a grafia", " Producao ", 0, "infCte.ide.tpAmb", 0, AmbienteProducao},
		{"vazio vira homologação", "", 0, "infCte.ide.tpAmb", 0, AmbienteHomologacao},
		{"tpAmb coerente", "producao", 1, "infCte.ide.tpAmb", 0, AmbienteProducao},
		{"tpAmb ausente é tolerado", "producao", 0, "infCte.ide.tpAmb", 0, AmbienteProducao},
		{"tpAmb contradiz", "producao", 2, "infCte.ide.tpAmb", http.StatusBadRequest, ""},
		{"ambiente que não existe", "prod", 0, "infCte.ide.tpAmb", http.StatusBadRequest, ""},
		// Documento sem tpAmb no contrato (NFS-e): não há o que conferir, e um
		// valor solto não pode virar recusa.
		{"sem campo de tpAmb", "producao", 2, "", 0, AmbienteProducao},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			amb := c.ambiente
			ok := AmbienteDoPedido(w, &amb, c.tpAmb, c.campo)
			if c.quero == 0 {
				if !ok {
					t.Fatalf("recusado: %s", w.Body)
				}
				if amb != c.normal {
					t.Errorf("ambiente = %q, quero %q: a sessão e o documento precisam ler o mesmo", amb, c.normal)
				}
				return
			}
			if ok {
				t.Fatal("passou")
			}
			if w.Code != c.quero {
				t.Errorf("status %d, quero %d", w.Code, c.quero)
			}
		})
	}
}

// A mensagem precisa dizer ONDE está a contradição: é o caminho no payload que
// o cliente vai corrigir, e ele muda por documento.
func TestAmbienteDoPedido_MensagemNomeiaOCampo(t *testing.T) {
	w := httptest.NewRecorder()
	amb := "producao"
	AmbienteDoPedido(w, &amb, 2, "infMDFe.ide.tpAmb")
	if !strings.Contains(w.Body.String(), "infMDFe.ide.tpAmb") {
		t.Errorf("a mensagem não nomeia o campo: %s", w.Body)
	}
}

func TestModalSuportado(t *testing.T) {
	casos := []struct {
		informado, rodoviario string
		quero                 bool
	}{
		{"01", "01", true},
		{"", "01", true}, // ausente: o default do documento é o rodoviário
		{"1", "1", true},
		{"", "1", true},
		{"03", "01", false},
		{"2", "1", false},
	}
	for _, c := range casos {
		w := httptest.NewRecorder()
		if got := ModalSuportado(w, c.informado, c.rodoviario); got != c.quero {
			t.Errorf("ModalSuportado(%q, %q) = %v", c.informado, c.rodoviario, got)
		}
		if !c.quero {
			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("status %d, quero 422", w.Code)
			}
			if !strings.Contains(w.Body.String(), "modal_nao_suportado") {
				t.Errorf("sem o código estável: %s", w.Body)
			}
		}
	}
}
