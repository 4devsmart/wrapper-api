package inifmt

import (
	"testing"
	"time"
)

// TestDataHoraNoFuso trava a semântica de data-hora dos documentos fiscais.
//
// O bug que motivou isto: o formatador antigo fazia time.Parse(RFC3339) e depois
// formatava o relógio de parede, DESCARTANDO o offset. Um pedido em UTC
// ("11:00Z", que é 08:00 em São Paulo) virava "11:00", e a lib carimbava o fuso
// do estado — resultando em 11:00-03:00, três horas adiantado. Sem erro nenhum.
func TestDataHoraNoFuso(t *testing.T) {
	sp := LocalDaUF("SP") // -03:00
	ac := LocalDaUF("AC") // -05:00

	casos := []struct {
		nome, entrada, quero string
		loc                  *time.Location
	}{
		// Com offset explícito, a entrada é um INSTANTE: converte.
		{"UTC para São Paulo", "2026-05-02T11:00:00Z", "02/05/2026 08:00:00", sp},
		{"mesmo fuso, não muda", "2026-05-02T08:00:00-03:00", "02/05/2026 08:00:00", sp},
		{"São Paulo para Acre", "2026-05-02T08:00:00-03:00", "02/05/2026 06:00:00", ac},
		{"vira o dia", "2026-05-02T01:00:00Z", "01/05/2026 22:00:00", sp},

		// Sem offset, a entrada é RELÓGIO DE PAREDE do emitente: não converte.
		{"ingênua ISO", "2026-05-02T08:00:00", "02/05/2026 08:00:00", sp},
		{"ingênua no Acre", "2026-05-02T08:00:00", "02/05/2026 08:00:00", ac},
		{"só data", "2026-05-02", "02/05/2026 00:00:00", sp},
		{"formato BR", "02/05/2026 08:00:00", "02/05/2026 08:00:00", sp},

		{"vazia", "", "", sp},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := DataHoraNoFuso(c.entrada, c.loc); got != c.quero {
				t.Errorf("DataHoraNoFuso(%q, %s) = %q, quero %q", c.entrada, c.loc, got, c.quero)
			}
		})
	}
}

// TestLocalDaUF confere a tabela oficial (a mesma do GetUTCUF do ACBr).
func TestLocalDaUF(t *testing.T) {
	casos := map[string]int{
		"AC": -5 * 3600,
		"AM": -4 * 3600, "RR": -4 * 3600, "RO": -4 * 3600, "MT": -4 * 3600, "MS": -4 * 3600,
		"SP": -3 * 3600, "RS": -3 * 3600, "PA": -3 * 3600, "DF": -3 * 3600,
		"":   -3 * 3600, // sem UF: Brasília, mesmo default do ACBr
		"xx": -3 * 3600, // desconhecida: idem
		"sp": -3 * 3600, // minúscula
	}
	for uf, quero := range casos {
		_, off := time.Now().In(LocalDaUF(uf)).Zone()
		if off != quero {
			t.Errorf("LocalDaUF(%q) offset = %ds, quero %ds", uf, off, quero)
		}
	}
}
