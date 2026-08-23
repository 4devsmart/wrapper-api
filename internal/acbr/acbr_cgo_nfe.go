//go:build acbrlib

// Binding cgo da ACBrLibNFe (libacbrnfe64, variante MT), RESTRITO a Distribuição
// DF-e + Manifestação do Destinatário, sem emissão. Reusa a máquina de sessão
// de dfeSession (acbr_cgo.go), como CT-e/MDF-e.
package acbr

/*
#include "acbrlib.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

type nfeLib struct{ s dfeSession }

func newNFeLib(cfg config.ACBr) *nfeLib {
	return &nfeLib{s: dfeSession{
		cfg: cfg,
		fns: natFns{
			inicializar: func(h *C.LibHandle, a, k *C.char) C.int { return C.NFE_Inicializar(h, a, k) },
			finalizar:   func(h C.LibHandle) C.int { return C.NFE_Finalizar(h) },
			config:      func(h C.LibHandle, s, k, v *C.char) C.int { return C.NFE_ConfigGravarValor(h, s, k, v) },
			ultRetorno:  func(h C.LibHandle, b *C.char, sz *C.int) C.int { return C.NFE_UltimoRetorno(h, b, sz) },
		},
		secCfg:  "NFe",
		secPDF:  "DANFE", // não usado (distribuição/manifestação não rasteriza); só p/ applyConfig
		schemas: cfg.SchemasPathNFe,
		slug:    "nfe",
		arqCfg:  "acbrlibnfe.ini",
	}}
}

func (l *nfeLib) Backend() string { return "cgo" }
func (l *nfeLib) Close() error    { return nil }

func (l *nfeLib) Version() (string, error) {
	res, err := l.s.run(TenantConfig{}, func(h C.LibHandle) (Result, error) {
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.NFE_Versao(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
	if err != nil {
		return "", err
	}
	if res.Codigo != 0 {
		return "", fmt.Errorf("NFE_Versao retornou %d: %s", res.Codigo, res.Resposta)
	}
	return res.Resposta, nil
}

// DistribuicaoDFe: mesma forma do CT-e (NF-e também exige AcUFAutor = cUF do autor).
func (l *nfeLib) DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cCNPJ := C.CString(p.CNPJCPF)
		defer C.free(unsafe.Pointer(cCNPJ))
		uf := C.int(p.UFAutor)
		var code int
		var resp string
		switch p.Modo {
		case DistChave:
			cv := C.CString(p.Chave)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.NFE_DistribuicaoDFePorChave(h, uf, cCNPJ, cv, b, sz)
			})
		case DistNSU:
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.NFE_DistribuicaoDFePorNSU(h, uf, cCNPJ, cv, b, sz)
			})
		default: // DistUltNSU
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.NFE_DistribuicaoDFePorUltNSU(h, uf, cCNPJ, cv, b, sz)
			})
		}
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Manifestar carrega o INI do evento de Manifestação do Destinatário e o
// transmite (NFE_CarregarEventoINI + NFE_EnviarEvento). A resposta é o retEvento.
func (l *nfeLib) Manifestar(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.NFE_CarregarEventoINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("NFE_CarregarEventoINI falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.NFE_EnviarEvento(h, 1, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}
