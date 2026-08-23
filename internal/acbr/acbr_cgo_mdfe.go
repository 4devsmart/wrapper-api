//go:build acbrlib

// Binding cgo da ACBrLibMDFe (libacbrmdfe64, variante MT). Reusa dfeSession;
// adiciona o Encerrar (evento obrigatório do MDF-e). Encerramento e
// cancelamento são eventos (CarregarEventoINI + EnviarEvento).
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

type mdfeLib struct{ s dfeSession }

func newMDFeLib(cfg config.ACBr) *mdfeLib {
	return &mdfeLib{s: dfeSession{
		cfg: cfg,
		fns: natFns{
			inicializar: func(h *C.LibHandle, a, k *C.char) C.int { return C.MDFE_Inicializar(h, a, k) },
			finalizar:   func(h C.LibHandle) C.int { return C.MDFE_Finalizar(h) },
			config:      func(h C.LibHandle, s, k, v *C.char) C.int { return C.MDFE_ConfigGravarValor(h, s, k, v) },
			ultRetorno:  func(h C.LibHandle, b *C.char, sz *C.int) C.int { return C.MDFE_UltimoRetorno(h, b, sz) },
		},
		secCfg:  "MDFe",
		secPDF:  "DAMDFe",
		schemas: cfg.SchemasPathMDFe,
		slug:    "mdfe",
		arqCfg:  "acbrlibmdfe.ini",
	}}
}

func (l *mdfeLib) Backend() string { return "cgo" }
func (l *mdfeLib) Close() error    { return nil }

func (l *mdfeLib) Version() (string, error) {
	res, err := l.s.run(TenantConfig{}, func(h C.LibHandle) (Result, error) {
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_Versao(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
	if err != nil {
		return "", err
	}
	if res.Codigo != 0 {
		return "", fmt.Errorf("MDFE_Versao retornou %d: %s", res.Codigo, res.Resposta)
	}
	return res.Resposta, nil
}

func (l *mdfeLib) Emitir(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.MDFE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("MDFE_CarregarINI falhou: %d", code)
		}
		// aLote=1, aImprimir=0, aSincrono=1 (resultado imediato).
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.MDFE_Enviar(h, 1, C.uchar(0), C.uchar(1), b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ObterXml(h, 0, b, sz) }); xml != "" {
				res.XML = xml
			}
			// PDF NÃO é gerado aqui: o render é o componente que mais crasha e,
			// neste ponto, o documento já foi autorizado pela SEFAZ. Um SIGSEGV
			// aqui deixava o DAMDFE emitido, o cliente com erro e nada
			// persistido. Agora sai sob demanda: ver RenderizarPDF.
		}
		return res, nil
	})
}

// MontarXML carrega o INI e devolve o XML montado, SEM transmitir à SEFAZ
// (MDFE_CarregarINI → MDFE_ObterXml). Preview/validação local.
func (l *mdfeLib) MontarXML(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.MDFE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("MDFE_CarregarINI falhou: %d", code)
		}
		code, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ObterXml(h, 0, b, sz) })
		return Result{Codigo: code, Resposta: xml, XML: xml}, nil
	})
}

// ValidarRegras carrega o INI e roda MDFE_ValidarRegrasdeNegocios: as rejeições
// que a SEFAZ daria, antecipadas localmente, sem certificado e sem rede. É a
// validação da geração. XSD fica de fora: ver o comentário da interface.
func (l *mdfeLib) ValidarRegras(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.MDFE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("MDFE_CarregarINI falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.MDFE_ValidarRegrasdeNegocios(h, b, sz)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Transmitir envia o XML montado na geração (MDFE_CarregarXML → MDFE_Enviar). A
// lib assina no envio, então é aqui, e só aqui, que o certificado é preciso.
func (l *mdfeLib) Transmitir(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.MDFE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("MDFE_CarregarXML falhou: %d", code)
		}
		// aLote=1, aImprimir=0, aSincrono=1.
		//
		// Aqui o parâmetro é decorativo: TMDFeRecepcao.InicializarServico
		// (ACBrMDFeWebServices.pas:764) faz `Sincrono := True` SEM condição, com
		// a nota "a partir de 01/07/2024 todos os MDF-e devem ser enviados no
		// modo síncrono". Passamos 1 para o código dizer a mesma coisa que a lib
		// faz, em vez de sugerir uma escolha que não existe.
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.MDFE_Enviar(h, 1, C.uchar(0), C.uchar(1), b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, x := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ObterXml(h, 0, b, sz) }); x != "" {
				res.XML = x
			}
		}
		return res, nil
	})
}

func (l *mdfeLib) Consultar(t TenantConfig, chave string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		c := C.CString(chave)
		defer C.free(unsafe.Pointer(c))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_Consultar(h, c, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Cancelar (tpEvento 110111) e Encerrar (tpEvento 110112) são eventos: o INI já
// vem montado pelo domínio; carrega via MDFE_CarregarEventoINI e transmite via
// MDFE_EnviarEvento.
func (l *mdfeLib) Cancelar(t TenantConfig, ini string) (Result, error)     { return l.evento(t, ini) }
func (l *mdfeLib) Encerrar(t TenantConfig, ini string) (Result, error)     { return l.evento(t, ini) }
func (l *mdfeLib) EnviarEvento(t TenantConfig, ini string) (Result, error) { return l.evento(t, ini) }

func (l *mdfeLib) evento(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.MDFE_CarregarEventoINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("MDFE_CarregarEventoINI falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_EnviarEvento(h, 1, b, sz) })
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ObterXml(h, 0, b, sz) }); xml != "" {
				res.XML = xml
			}
		}
		return res, nil
	})
}

// RenderizarPDF gera o DAMDFE a partir do XML já autorizado (CarregarXML →
// SalvarPDF), fora do caminho de emissão. É o que permite o PDF sob demanda:
// perder o render deixou de significar perder o documento.
func (l *mdfeLib) RenderizarPDF(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.MDFE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code}, fmt.Errorf("MDFE_CarregarXML falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_SalvarPDF(h, b, sz) })
		pdf := pdfDecodificado(resp)
		if len(pdf) == 0 {
			return Result{Codigo: code, Resposta: resp}, fmt.Errorf("MDFE_SalvarPDF não devolveu PDF")
		}
		return Result{Codigo: code, PDF: pdf}, nil
	})
}

func (l *mdfeLib) ObterPDF(_ TenantConfig, _ string) (Result, error) {
	return Result{}, fmt.Errorf("ObterPDF sob demanda não suportado para MDF-e; use o PDF da emissão")
}

// SalvarEventoPDF gera o DAMDFE do evento a partir do XML do MDF-e + o XML do
// evento (conteúdo, não path). PDF em Result.PDF, como o MDFE_SalvarPDF.
func (l *mdfeLib) SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cDoc := C.CString(xmlDoc)
		cEv := C.CString(xmlEvento)
		defer C.free(unsafe.Pointer(cDoc))
		defer C.free(unsafe.Pointer(cEv))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.MDFE_SalvarEventoPDF(h, cDoc, cEv, b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			res.PDF = pdfDecodificado(resp)
		}
		return res, nil
	})
}

// DistribuicaoDFe consulta a Distribuição DF-e do MDF-e. Sessão direta (sem
// CarregarINI). MDF-e NÃO recebe AcUFAutor (p.UFAutor é ignorado de propósito).
func (l *mdfeLib) DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cCNPJ := C.CString(p.CNPJCPF)
		defer C.free(unsafe.Pointer(cCNPJ))
		var code int
		var resp string
		switch p.Modo {
		case DistChave:
			cv := C.CString(p.Chave)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.MDFE_DistribuicaoDFePorChave(h, cCNPJ, cv, b, sz)
			})
		case DistNSU:
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.MDFE_DistribuicaoDFePorNSU(h, cCNPJ, cv, b, sz)
			})
		default: // DistUltNSU
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.MDFE_DistribuicaoDFePorUltNSU(h, cCNPJ, cv, b, sz)
			})
		}
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// StatusServico consulta o status do serviço da SEFAZ-MDFe.
func (l *mdfeLib) StatusServico(t TenantConfig) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_StatusServico(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarRecibo consulta o processamento de um lote pelo recibo.
func (l *mdfeLib) ConsultarRecibo(t TenantConfig, recibo string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cr := C.CString(recibo)
		defer C.free(unsafe.Pointer(cr))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ConsultarRecibo(h, cr, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultaNaoEncerrados lista os MDF-e do CNPJ ainda não encerrados.
func (l *mdfeLib) ConsultaNaoEncerrados(t TenantConfig, cnpj string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cc := C.CString(cnpj)
		defer C.free(unsafe.Pointer(cc))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ConsultaMDFeNaoEnc(h, cc, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

func (l *mdfeLib) XmlParaIni(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.MDFE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, nil
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.MDFE_ObterIni(h, 0, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}
