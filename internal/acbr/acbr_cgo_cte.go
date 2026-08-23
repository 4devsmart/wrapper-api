//go:build acbrlib

// Binding cgo da ACBrLibCTe (libacbrcte64, variante MT). Reusa a máquina de
// sessão de dfeSession (acbr_cgo.go); só as operações são específicas do CT-e.
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

type cteLib struct{ s dfeSession }

func newCTeLib(cfg config.ACBr) *cteLib {
	return &cteLib{s: dfeSession{
		cfg: cfg,
		fns: natFns{
			inicializar: func(h *C.LibHandle, a, k *C.char) C.int { return C.CTE_Inicializar(h, a, k) },
			finalizar:   func(h C.LibHandle) C.int { return C.CTE_Finalizar(h) },
			config:      func(h C.LibHandle, s, k, v *C.char) C.int { return C.CTE_ConfigGravarValor(h, s, k, v) },
			ultRetorno:  func(h C.LibHandle, b *C.char, sz *C.int) C.int { return C.CTE_UltimoRetorno(h, b, sz) },
		},
		secCfg:  "CTe",
		secPDF:  "DACTe",
		schemas: cfg.SchemasPathCTe,
		slug:    "cte",
		arqCfg:  "acbrlibcte.ini",
	}}
}

func (l *cteLib) Backend() string { return "cgo" }
func (l *cteLib) Close() error    { return nil }

func (l *cteLib) Version() (string, error) {
	res, err := l.s.run(TenantConfig{}, func(h C.LibHandle) (Result, error) {
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_Versao(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
	if err != nil {
		return "", err
	}
	if res.Codigo != 0 {
		return "", fmt.Errorf("CTE_Versao retornou %d: %s", res.Codigo, res.Resposta)
	}
	return res.Resposta, nil
}

func (l *cteLib) Emitir(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.CTE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("CTE_CarregarINI falhou: %d", code)
		}
		// aLote=1, aImprimir=0 (PDF é gerado adiante, gated por DANFSeLocal).
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.CTE_Enviar(h, 1, C.uchar(0), C.uchar(1), b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ObterXml(h, 0, b, sz) }); xml != "" {
				res.XML = xml
			}
			// PDF NÃO é gerado aqui: o render é o componente que mais crasha e,
			// neste ponto, o documento já foi autorizado pela SEFAZ. Um SIGSEGV
			// aqui deixava o DACTE emitido, o cliente com erro e nada
			// persistido. Agora sai sob demanda — ver RenderizarPDF.
		}
		return res, nil
	})
}

// MontarXML carrega o INI e devolve o XML montado, SEM transmitir à SEFAZ
// (CTE_CarregarINI → CTE_ObterXml). Preview/validação local.
func (l *cteLib) MontarXML(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.CTE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("CTE_CarregarINI falhou: %d", code)
		}
		code, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ObterXml(h, 0, b, sz) })
		return Result{Codigo: code, Resposta: xml, XML: xml}, nil
	})
}

// ValidarRegras carrega o INI e roda CTE_ValidarRegrasdeNegocios: as rejeições
// que a SEFAZ daria, antecipadas localmente, sem certificado e sem rede. É a
// validação da geração. XSD fica de fora — ver o comentário da interface.
func (l *cteLib) ValidarRegras(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.CTE_CarregarINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("CTE_CarregarINI falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.CTE_ValidarRegrasdeNegocios(h, b, sz)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Transmitir envia o XML montado na geração (CTE_CarregarXML → CTE_Enviar). A lib
// assina no envio, então é aqui — e só aqui — que o certificado é necessário.
func (l *cteLib) Transmitir(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.CTE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("CTE_CarregarXML falhou: %d", code)
		}
		// aLote=1, aImprimir=0 (PDF sai sob demanda), aSincrono=1.
		//
		// A SEFAZ desativou a recepção assíncrona: hoje é tudo síncrono, e o
		// protocolo volta na mesma chamada. Na 4.00 o parâmetro nem decide —
		// TCTeRecepcao.InicializarServico (ACBrCTeWebServices.pas:875) força
		// Sincrono para VersaoDF >= ve400, escolhe o endpoint CTeRecepcaoSincV4
		// e recusa lote com mais de um CT-e. Passamos 1 para que o comportamento
		// seja o mesmo caso alguém rode com uma versão anterior.
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.CTE_Enviar(h, 1, C.uchar(0), C.uchar(1), b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, x := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ObterXml(h, 0, b, sz) }); x != "" {
				res.XML = x
			}
		}
		return res, nil
	})
}

func (l *cteLib) Consultar(t TenantConfig, chave string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		c := C.CString(chave)
		defer C.free(unsafe.Pointer(c))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_Consultar(h, c, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Cancelar: o INI é o evento de cancelamento (tpEvento 110111, formato indexado
// [EVENTO001]). Mesmo mecanismo de qualquer evento.
func (l *cteLib) Cancelar(t TenantConfig, ini string) (Result, error) {
	return l.enviarEventoINI(t, ini)
}

// CartaCorrecao: o INI é a CC-e (tpEvento 110110, [EVENTO001]+[DETEVENTO001..]).
func (l *cteLib) CartaCorrecao(t TenantConfig, ini string) (Result, error) {
	return l.enviarEventoINI(t, ini)
}

// EnviarEvento: evento genérico do CT-e (EPEC, comprovante de entrega, etc.).
func (l *cteLib) EnviarEvento(t TenantConfig, ini string) (Result, error) {
	return l.enviarEventoINI(t, ini)
}

// enviarEventoINI carrega o INI do evento (CTE_CarregarEventoINI) e o transmite
// (CTE_EnviarEvento). Comum a cancelamento e CC-e — a diferença é só o tpEvento
// no INI. Em sucesso, anexa o XML do protocolo de evento.
func (l *cteLib) enviarEventoINI(t TenantConfig, ini string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		if code := int(C.CTE_CarregarEventoINI(h, cIni)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, fmt.Errorf("CTE_CarregarEventoINI falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_EnviarEvento(h, 1, b, sz) })
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ObterXml(h, 0, b, sz) }); xml != "" {
				res.XML = xml
			}
		}
		return res, nil
	})
}

// RenderizarPDF gera o DACTE a partir do XML já autorizado (CarregarXML →
// SalvarPDF), fora do caminho de emissão. É o que permite o PDF sob demanda:
// perder o render deixou de significar perder o documento.
func (l *cteLib) RenderizarPDF(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.CTE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code}, fmt.Errorf("CTE_CarregarXML falhou: %d", code)
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_SalvarPDF(h, b, sz) })
		pdf := pdfDecodificado(resp)
		if len(pdf) == 0 {
			return Result{Codigo: code, Resposta: resp}, fmt.Errorf("CTE_SalvarPDF não devolveu PDF")
		}
		return Result{Codigo: code, PDF: pdf}, nil
	})
}

// ObterPDF sob demanda não é suportado para CT-e (o DACTE é gerado na emissão e
// servido do storage). Implementado para satisfazer a interface Servico.
func (l *cteLib) ObterPDF(_ TenantConfig, _ string) (Result, error) {
	return Result{}, fmt.Errorf("ObterPDF sob demanda não suportado para CT-e; use o PDF da emissão")
}

// SalvarEventoPDF gera o DACTE do evento a partir do XML do CT-e + o XML do
// evento (ambos passados como conteúdo, não path). Devolve o PDF em Result.PDF
// (base64 → bytes), mesmo padrão do CTE_SalvarPDF.
func (l *cteLib) SalvarEventoPDF(t TenantConfig, xmlDoc, xmlEvento string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cDoc := C.CString(xmlDoc)
		cEv := C.CString(xmlEvento)
		defer C.free(unsafe.Pointer(cDoc))
		defer C.free(unsafe.Pointer(cEv))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.CTE_SalvarEventoPDF(h, cDoc, cEv, b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			res.PDF = pdfDecodificado(resp)
		}
		return res, nil
	})
}

// DistribuicaoDFe consulta a Distribuição DF-e do CT-e. Sessão direta (sem
// CarregarINI): Inicializar → config → CTE_DistribuicaoDFePor{UltNSU,NSU,Chave}.
// CT-e exige AcUFAutor (cUF do autor). A resposta é o INI do lote.
func (l *cteLib) DistribuicaoDFe(t TenantConfig, p DistDFeParams) (Result, error) {
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
				return C.CTE_DistribuicaoDFePorChave(h, uf, cCNPJ, cv, b, sz)
			})
		case DistNSU:
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.CTE_DistribuicaoDFePorNSU(h, uf, cCNPJ, cv, b, sz)
			})
		default: // DistUltNSU
			cv := C.CString(p.NSU)
			defer C.free(unsafe.Pointer(cv))
			code, resp = l.s.callBuf(h, func(b *C.char, sz *C.int) C.int {
				return C.CTE_DistribuicaoDFePorUltNSU(h, uf, cCNPJ, cv, b, sz)
			})
		}
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// StatusServico consulta o status do serviço da SEFAZ-CTe.
func (l *cteLib) StatusServico(t TenantConfig) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_StatusServico(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarRecibo consulta o processamento de um lote pelo recibo.
func (l *cteLib) ConsultarRecibo(t TenantConfig, recibo string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cr := C.CString(recibo)
		defer C.free(unsafe.Pointer(cr))
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ConsultarRecibo(h, cr, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultaCadastro consulta o cadastro de um contribuinte na UF.
func (l *cteLib) ConsultaCadastro(t TenantConfig, uf, nDocumento string, ehIE bool) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cUF := C.CString(uf)
		cDoc := C.CString(nDocumento)
		defer C.free(unsafe.Pointer(cUF))
		defer C.free(unsafe.Pointer(cDoc))
		nIE := C.uchar(0)
		if ehIE {
			nIE = 1
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ConsultaCadastro(h, cUF, cDoc, nIE, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

func (l *cteLib) XmlParaIni(t TenantConfig, xml string) (Result, error) {
	return l.s.run(t, func(h C.LibHandle) (Result, error) {
		cXML := C.CString(xml)
		defer C.free(unsafe.Pointer(cXML))
		if code := int(C.CTE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: l.s.ultRet(h, 0)}, nil
		}
		code, resp := l.s.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.CTE_ObterIni(h, 0, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
}
