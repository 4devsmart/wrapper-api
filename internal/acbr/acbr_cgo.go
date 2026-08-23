//go:build acbrlib

// Implementação real do binding via cgo, ligando a variante MultiThread (MT)
// da ACBrLibNFSe (libacbrnfse64.so).
//
// Fluxo por operação: NFSE_Inicializar (handle MT, exige caminho de config) →
// aplica config (SSL Linux + certificado + chaves do tenant) → CarregarINI /
// Emitir / Consultar → NFSE_Finalizar.
//
// ATENÇÃO: o agendador do Go troca goroutines de thread; toda sequência de
// chamadas nativas fixa a thread (runtime.LockOSThread) e opera sobre um
// handle dedicado. (Pool de handles por empresa fica para a Fase 5.)
//
// Build: CGO_ENABLED=1 go build -tags acbrlib ./...
package acbr

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -lacbrnfse64 -lacbrcte64 -lacbrmdfe64 -lacbrnfe64 -lacbrboleto64
#include "acbrlib.h"
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

// bufferLen é o tamanho inicial do buffer de resposta (mesma convenção do SDK
// oficial: BUFFER_LEN=256). Se a resposta for maior, esTamanho retorna o
// tamanho real e relemos via NFSE_UltimoRetorno.
const bufferLen = 256

// Valores de SSL obrigatórios no Linux (ver enums do ACBr / SDK C#):
// cryOpenSSL=1, httpOpenSSL=3, xsLibXml2=4.
const (
	sslCryptOpenSSL = "1"
	sslHttpOpenSSL  = "3"
	sslXmlLibXml2   = "4"
)

type cgoLib struct {
	cfg config.ACBr
}

// cgoLib é o serviço NFSe (CTe/MDFe/NFe/Boleto têm tipos próprios).
var _ NFSeServico = (*cgoLib)(nil)

// newServicos (cgo) liga o binding real da NFSe. CTe/MDFe ainda usam o
// placeholder indisponível — os bindings reais (libacbrcte64/libacbrmdfe64)
// entram na Fase 2 (build dos .so + cgo CTE_*/MDFE_*).
func newServicos(cfg config.ACBr) *Servicos {
	return &Servicos{
		NFSe:   &cgoLib{cfg: cfg},
		CTe:    newCTeLib(cfg),
		MDFe:   newMDFeLib(cfg),
		NFe:    newNFeLib(cfg),
		Boleto: newBoletoLib(cfg),
	}
}

// natFns são as funções nativas COMUNS a todas as ACBrLib (mesma assinatura;
// só muda o prefixo), como closures sobre os símbolos C. Permite compartilhar a
// máquina de sessão entre CTe e MDFe (definidas em acbr_cgo_cte/mdfe.go).
type natFns struct {
	inicializar func(h *C.LibHandle, arqConfig, chave *C.char) C.int
	finalizar   func(h C.LibHandle) C.int
	config      func(h C.LibHandle, sec, key, val *C.char) C.int
	ultRetorno  func(h C.LibHandle, buf *C.char, size *C.int) C.int
}

// dfeSession é a base compartilhada dos bindings de documentos modelo SEFAZ
// (CT-e/MDF-e): ciclo Inicializar → config (SSL + cert + paths) → Finalizar,
// com a thread fixada. Cada serviço (acbr_cgo_cte/mdfe.go) embute esta base e
// implementa suas operações (Enviar/Consultar/EnviarEvento/SalvarPDF).
type dfeSession struct {
	cfg     config.ACBr
	fns     natFns
	secCfg  string // seção de config p/ PathSalvar/PathSchemas (ex.: "CTe")
	secPDF  string // seção p/ PathPDF (ex.: "DACTe")
	schemas string // diretório de schemas XSD do serviço
	slug    string // prefixo do dir/arquivo de sessão (ex.: "cte")
	arqCfg  string // nome do arquivo de config (ex.: "acbrlibcte.ini")
}

func (s *dfeSession) run(t TenantConfig, fn func(h C.LibHandle) (Result, error)) (Result, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	base := s.cfg.IniBasePath
	if base == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return Result{}, fmt.Errorf("criando diretório base: %w", err)
	}
	sessionDir, err := os.MkdirTemp(base, s.slug+"-"+tenantSlug(t.CNPJ)+"-")
	if err != nil {
		return Result{}, fmt.Errorf("criando diretório de sessão: %w", err)
	}
	defer os.RemoveAll(sessionDir)
	cfgPath := filepath.Join(sessionDir, s.arqCfg)

	if s.cfg.LogPath != "" {
		_ = os.MkdirAll(s.cfg.LogPath, 0o755)
		nivel := s.cfg.LogNivel
		if nivel == "" {
			nivel = "4"
		}
		_ = os.WriteFile(cfgPath, []byte("[Principal]\nLogNivel="+nivel+"\nLogPath="+s.cfg.LogPath+"\n"), 0o644)
	}

	cArq := C.CString(cfgPath)
	cKey := C.CString("")
	defer C.free(unsafe.Pointer(cArq))
	defer C.free(unsafe.Pointer(cKey))

	var h C.LibHandle
	if ret := s.fns.inicializar(&h, cArq, cKey); ret != 0 {
		return Result{}, fmt.Errorf("%s Inicializar falhou: código %d", s.secCfg, int(ret))
	}
	defer s.fns.finalizar(h)

	if err := s.applyConfig(h, t, sessionDir); err != nil {
		return Result{}, err
	}
	return fn(h)
}

// timeZonePorUF é o modo tzPCN do ACBr (TTimeZoneModoDeteccao): o offset do
// documento vem da UF, não do relógio do servidor.
const timeZonePorUF = "1"

// applyConfig grava SSL (Linux) + certificado A1 + paths graváveis + schemas +
// as chaves adicionais do tenant.
func (s *dfeSession) applyConfig(h C.LibHandle, t TenantConfig, savePath string) error {
	kvs := []ConfigKV{
		{"DFe", "SSLCryptLib", sslCryptOpenSSL},
		{"DFe", "SSLHttpLib", sslHttpOpenSSL},
		{"DFe", "SSLXmlSignLib", sslXmlLibXml2},
		// Fuso do documento pela UF do emitente, não pelo relógio do servidor.
		// O default da lib (tzSistema) carimba o fuso do PROCESSO — e o container
		// roda em UTC, então todo dhEmi saía com +00:00: um CT-e emitido às 08:00
		// em São Paulo virava 08:00Z, três horas adiantado. Com tzPCN a lib usa a
		// tabela oficial por UF (AC -05, AM/RR/RO/MT/MS -04, demais -03), com
		// horário de verão. Ver GetUTCUF em ACBrUtil.DateTime.pas.
		{"DFe", "TimeZone.Modo", timeZonePorUF},
		{s.secCfg, "PathSalvar", savePath},
		{s.secPDF, "PathPDF", savePath},
	}
	if s.schemas != "" {
		kvs = append(kvs, ConfigKV{s.secCfg, "PathSchemas", s.schemas})
	}
	if t.PFXBase64 != "" {
		kvs = append(kvs,
			ConfigKV{"DFe", "DadosPFX", t.PFXBase64},
			ConfigKV{"DFe", "Senha", t.SenhaPFX},
		)
	}
	kvs = append(kvs, t.Config...)
	for _, kv := range kvs {
		cs, ck, cv := C.CString(kv.Section), C.CString(kv.Key), C.CString(kv.Value)
		ret := s.fns.config(h, cs, ck, cv)
		C.free(unsafe.Pointer(cs))
		C.free(unsafe.Pointer(ck))
		C.free(unsafe.Pointer(cv))
		if ret != 0 {
			return fmt.Errorf("ConfigGravarValor [%s]%s falhou: %d", kv.Section, kv.Key, int(ret))
		}
	}
	return nil
}

// callBuf aplica a convenção de buffer da ACBrLib usando a UltimoRetorno do
// serviço (via vtable). Igual à callBuffer do NFSe, mas genérica.
func (s *dfeSession) callBuf(h C.LibHandle, call func(buf *C.char, size *C.int) C.int) (int, string) {
	buf := make([]byte, bufferLen)
	size := C.int(bufferLen)
	ret := int(call((*C.char)(unsafe.Pointer(&buf[0])), &size))
	var resp string
	if int(size) > bufferLen {
		resp = s.ultRet(h, int(size))
	} else {
		resp = cstr(buf)
	}
	if ret != 0 && resp == "" {
		resp = s.ultRet(h, 0)
	}
	return ret, resp
}

func (s *dfeSession) ultRet(h C.LibHandle, hint int) string {
	n := hint
	if n < bufferLen {
		n = bufferLen
	}
	buf := make([]byte, n+1)
	size := C.int(n)
	s.fns.ultRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	if int(size) > n {
		n = int(size)
		buf = make([]byte, n+1)
		size = C.int(n)
		s.fns.ultRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	}
	return cstr(buf)
}

// pdfDecodificado interpreta a resposta do SalvarPDF (base64 ou binário).
func pdfDecodificado(resp string) []byte {
	resp = strings.TrimSpace(resp)
	if resp == "" {
		return nil
	}
	if pdf, err := base64.StdEncoding.DecodeString(resp); err == nil && bytes.HasPrefix(pdf, []byte("%PDF")) {
		return pdf
	}
	if bytes.HasPrefix([]byte(resp), []byte("%PDF")) {
		return []byte(resp)
	}
	return nil
}

func (l *cgoLib) Backend() string { return "cgo" }

func (l *cgoLib) Version() (string, error) {
	res, err := l.withSession(TenantConfig{}, func(h C.LibHandle) (Result, error) {
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_Versao(h, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
	if err != nil {
		return "", err
	}
	if res.Codigo != 0 {
		return "", fmt.Errorf("NFSE_Versao retornou %d: %s", res.Codigo, res.Resposta)
	}
	return res.Resposta, nil
}

func (l *cgoLib) Emitir(t TenantConfig, iniNFSe string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		if code, resp := configCarregarINI(h, iniNFSe); code != 0 {
			return Result{Codigo: code, Resposta: resp}, fmt.Errorf("NFSE_CarregarINI falhou: %d", code)
		}
		// aLote: número do lote de envio (X111 se vazio). "1" para emissão
		// individual; lotes/numeração avançada ficam para iterações futuras.
		lote, freeLote := cstrFree("1")
		defer freeLote()
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			// aModoEnvio=0 (automático conforme provedor), aImprimir=0.
			return C.NFSE_Emitir(h, lote, 0, C.uchar(0), buf, size)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			// XML do documento gerado (índice 0).
			if _, xml := callBuffer(h, func(buf *C.char, size *C.int) C.int {
				return C.NFSE_ObterXml(h, 0, buf, size)
			}); xml != "" {
				res.XML = xml
			}
			// O PDF NÃO é gerado aqui. O render (FortesReport/GTK2) é o que mais
			// crasha, e neste ponto o documento JÁ foi autorizado pela prefeitura:
			// um SIGSEGV aqui deixava a nota emitida, o cliente com erro e nada
			// persistido. Agora ele acontece sob demanda, a partir do XML
			// autorizado — ver RenderizarPDF.
		}
		return res, nil
	})
}

// RenderizarPDF gera o DANFSE a partir do XML já autorizado (CarregarXML →
// SalvarPDF), fora do caminho de emissão. Exige o .so com widgetset GTK2 sob
// Xvfb (o nogui não rasteriza: TBitmap sem backend → SIGSEGV).
func (l *cgoLib) RenderizarPDF(t TenantConfig, xml string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cXML, free := cstrFree(xml)
		defer free()
		if code := int(C.NFSE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: ultimoRetorno(h, 0)},
				fmt.Errorf("NFSE_CarregarXML falhou: %d", code)
		}
		pdf := salvarPDF(h)
		if len(pdf) == 0 {
			return Result{}, fmt.Errorf("NFSE_SalvarPDF não devolveu PDF")
		}
		return Result{PDF: pdf}, nil
	})
}

// ConsultarDPSPorChave pergunta ao provedor se a DPS de chave informada virou
// NFS-e (no Padrão Nacional, GET /dps/{chave} no ADN).
//
// É o que fecha o modelo sem estado para a NFS-e: a fase 1 devolve o
// identificador da DPS antes de qualquer envio, e é com ele que o cliente
// descobre o desfecho de uma transmissão cuja resposta se perdeu — em vez de
// reenviar e arriscar duplicar.
func (l *cgoLib) ConsultarDPSPorChave(t TenantConfig, chaveDPS string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		c, free := cstrFree(chaveDPS)
		defer free()
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarDPSPorChave(h, c, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// MontarXML carrega o INI e devolve o XML montado, SEM transmitir ao provedor.
// Para a NFS-e o documento gerado é o RPS/DPS (NFSE_ObterXmlRps); o NFSE_ObterXml
// devolve a NFS-e final, que só existe após a autorização (vazio no preview).
func (l *cgoLib) MontarXML(t TenantConfig, ini string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		if code, resp := configCarregarINI(h, ini); code != 0 {
			return Result{Codigo: code, Resposta: resp}, fmt.Errorf("NFSE_CarregarINI falhou: %d", code)
		}
		code, xml := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ObterXmlRps(h, 0, buf, size)
		})
		if xml == "" { // fallback: alguns provedores expõem só o ObterXml
			code, xml = callBuffer(h, func(buf *C.char, size *C.int) C.int {
				return C.NFSE_ObterXml(h, 0, buf, size)
			})
		}
		return Result{Codigo: code, Resposta: xml, XML: xml}, nil
	})
}

// ValidarRegras não existe para a NFS-e: o ACBrNFSeX não exporta validação de
// regras de negócio (CT-e e MDF-e exportam). Falha explícita em vez de devolver
// "ok" silencioso, que faria o cliente confiar numa validação que não ocorreu.
func (l *cgoLib) ValidarRegras(TenantConfig, string) (Result, error) {
	return Result{}, ErrNaoSuportado
}

// Transmitir envia a DPS montada na fase 1 (NFSE_CarregarXML → NFSE_Emitir). É
// aqui que o provedor assina — a NFS-e não expõe assinatura separada, então a
// fase 1 entrega a DPS crua e o certificado só é necessário neste ponto.
func (l *cgoLib) Transmitir(t TenantConfig, xml string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cXML, free := cstrFree(xml)
		defer free()
		if code := int(C.NFSE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: ultimoRetorno(h, 0)},
				fmt.Errorf("NFSE_CarregarXML falhou: %d", code)
		}
		lote, freeLote := cstrFree("1")
		defer freeLote()
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			// aModoEnvio=0 (automático conforme provedor), aImprimir=0.
			return C.NFSE_Emitir(h, lote, 0, C.uchar(0), buf, size)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, x := callBuffer(h, func(buf *C.char, size *C.int) C.int {
				return C.NFSE_ObterXml(h, 0, buf, size)
			}); x != "" {
				res.XML = x
			}
		}
		return res, nil
	})
}

// Cancelar envia o evento de cancelamento. eInfCancelamento é o INI
// [CancelarNFSe] (passado diretamente, não via CarregarINI). Em caso de
// sucesso, tenta capturar o XML do evento gerado (ObterXml índice 0).
func (l *cgoLib) Cancelar(t TenantConfig, iniCancel string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cIni, freeIni := cstrFree(iniCancel)
		defer freeIni()
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_Cancelar(h, cIni, buf, size)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := callBuffer(h, func(buf *C.char, size *C.int) C.int {
				return C.NFSE_ObterXml(h, 0, buf, size)
			}); xml != "" {
				res.XML = xml
			}
		}
		return res, nil
	})
}

// salvarPDF chama NFSE_SalvarPDF: gera o DANFSE da nota emitida/carregada e
// devolve o PDF (decodificado de base64). Best-effort: nil se nao gerar um PDF
// valido. A resposta e grande (base64), relida via UltimoRetorno por callBuffer.
func salvarPDF(h C.LibHandle) []byte {
	code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
		return C.NFSE_SalvarPDF(h, buf, size)
	})
	if code != 0 || resp == "" {
		return nil
	}
	resp = strings.TrimSpace(resp)
	if pdf, err := base64.StdEncoding.DecodeString(resp); err == nil && bytes.HasPrefix(pdf, []byte("%PDF")) {
		return pdf
	}
	if bytes.HasPrefix([]byte(resp), []byte("%PDF")) { // ja veio binario
		return []byte(resp)
	}
	return nil
}

// iniValue extrai "chave=valor" de um texto INI simples (primeira ocorrência).
func iniValue(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SubstituirNFSe carrega a DPS substituta (NFSE_CarregarINI) e transmite a
// substituição da NFS-e antiga (NFSE_SubstituirNFSe). O componente exige a nota
// nova carregada antes. Em sucesso, captura o XML (e o PDF, se DANFSeLocal).
func (l *cgoLib) SubstituirNFSe(t TenantConfig, iniNovaDPS string, sub SubstituicaoNFSe) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		if code, resp := configCarregarINI(h, iniNovaDPS); code != 0 {
			return Result{Codigo: code, Resposta: resp}, fmt.Errorf("NFSE_CarregarINI (DPS substituta) falhou: %d", code)
		}
		cNum := C.CString(sub.NumeroNFSe)
		cSerie := C.CString(sub.SerieNFSe)
		cCod := C.CString(sub.CodigoCancelamento)
		cMot := C.CString(sub.MotivoCancelamento)
		cLote := C.CString(sub.NumeroLote)
		cVerif := C.CString(sub.CodigoVerificacao)
		defer C.free(unsafe.Pointer(cNum))
		defer C.free(unsafe.Pointer(cSerie))
		defer C.free(unsafe.Pointer(cCod))
		defer C.free(unsafe.Pointer(cMot))
		defer C.free(unsafe.Pointer(cLote))
		defer C.free(unsafe.Pointer(cVerif))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_SubstituirNFSe(h, cNum, cSerie, cCod, cMot, cLote, cVerif, buf, size)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if _, xml := callBuffer(h, func(buf *C.char, size *C.int) C.int {
				return C.NFSE_ObterXml(h, 0, buf, size)
			}); xml != "" {
				res.XML = xml
			}
			// PDF sob demanda (ver RenderizarPDF): não renderiza no caminho da
			// substituição, que também já transmitiu.
		}
		return res, nil
	})
}

func (l *cgoLib) Consultar(t TenantConfig, chave string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cChave := C.CString(chave)
		defer C.free(unsafe.Pointer(cChave))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarNFSePorChave(h, cChave, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarDFe consulta a Distribuição DF-e do ADN (Padrão Nacional) por NSU.
// O CNPJ é resolvido pela config do componente (aplicada em withSession), não
// pelo parâmetro. A resposta é o INI de [ConsultarDFe] + [Arquivo###].
func (l *cgoLib) ConsultarDFe(t TenantConfig, nsu int) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarDFe(h, C.int(nsu), buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarPorNumero consulta a NFS-e pelo número (paginação por pagina≥1).
func (l *cgoLib) ConsultarPorNumero(t TenantConfig, numero string, pagina int) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cn := C.CString(numero)
		defer C.free(unsafe.Pointer(cn))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarNFSePorNumero(h, cn, C.int(pagina), buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarPorFaixa consulta NFS-e numa faixa de números (paginação).
func (l *cgoLib) ConsultarPorFaixa(t TenantConfig, inicial, final string, pagina int) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		ci := C.CString(inicial)
		cf := C.CString(final)
		defer C.free(unsafe.Pointer(ci))
		defer C.free(unsafe.Pointer(cf))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarNFSePorFaixa(h, ci, cf, C.int(pagina), buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarPorRps consulta a NFS-e a partir do RPS que a originou.
func (l *cgoLib) ConsultarPorRps(t TenantConfig, numeroRps, serie, tipo, codVerificacao string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cn := C.CString(numeroRps)
		cs := C.CString(serie)
		ct := C.CString(tipo)
		cv := C.CString(codVerificacao)
		defer C.free(unsafe.Pointer(cn))
		defer C.free(unsafe.Pointer(cs))
		defer C.free(unsafe.Pointer(ct))
		defer C.free(unsafe.Pointer(cv))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarNFSePorRps(h, cn, cs, ct, cv, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarSituacao consulta a situação de um lote pelo protocolo/numLote.
func (l *cgoLib) ConsultarSituacao(t TenantConfig, protocolo, numLote string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cp := C.CString(protocolo)
		cl := C.CString(numLote)
		defer C.free(unsafe.Pointer(cp))
		defer C.free(unsafe.Pointer(cl))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarSituacao(h, cp, cl, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// ConsultarLoteRps consulta o resultado de um lote de RPS pelo protocolo/numLote.
func (l *cgoLib) ConsultarLoteRps(t TenantConfig, protocolo, numLote string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cp := C.CString(protocolo)
		cl := C.CString(numLote)
		defer C.free(unsafe.Pointer(cp))
		defer C.free(unsafe.Pointer(cl))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ConsultarLoteRps(h, cp, cl, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

func (l *cgoLib) ObterPDF(t TenantConfig, chave string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cChave := C.CString(chave)
		defer C.free(unsafe.Pointer(cChave))
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ObterDANFSE(h, cChave, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// XmlParaIni carrega um XML de NFSe e devolve o INI canônico (CarregarXML +
// ObterIni). Ferramenta de diagnóstico para descobrir o conjunto completo de
// campos que a lib espera.
func (l *cgoLib) XmlParaIni(t TenantConfig, xml string) (Result, error) {
	return l.withSession(t, func(h C.LibHandle) (Result, error) {
		cXML, freeXML := cstrFree(xml)
		defer freeXML()
		if code := int(C.NFSE_CarregarXML(h, cXML)); code != 0 {
			return Result{Codigo: code, Resposta: ultimoRetorno(h, 0)}, nil
		}
		code, resp := callBuffer(h, func(buf *C.char, size *C.int) C.int {
			return C.NFSE_ObterIni(h, 0, buf, size)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

func (l *cgoLib) Close() error { return nil } // sessões são por-chamada; nada a fechar

// withSession inicializa um handle MT com o config do tenant, aplica a
// configuração, executa fn e finaliza. Fixa a thread do SO durante toda a
// sequência (a lib não é reentrante por handle).
func (l *cgoLib) withSession(t TenantConfig, fn func(h C.LibHandle) (Result, error)) (Result, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	base := l.cfg.IniBasePath
	if base == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return Result{}, fmt.Errorf("criando diretório base: %w", err)
	}
	// Diretório efêmero por emissão: guarda o config (com o cert em texto puro)
	// e os XMLs/JSONs que o ACBr salva (PathSalvar). Apagado por inteiro ao
	// fim da sessão — nada sensível fica em repouso.
	sessionDir, err := os.MkdirTemp(base, "nfse-"+tenantSlug(t.CNPJ)+"-")
	if err != nil {
		return Result{}, fmt.Errorf("criando diretório de sessão: %w", err)
	}
	defer os.RemoveAll(sessionDir)
	cfgPath := filepath.Join(sessionDir, "acbrlibnfse.ini")

	// Log de depuração do ACBr: o logger é inicializado no Inicializar lendo do
	// arquivo de config, então gravamos [Principal] LogNivel/LogPath ANTES.
	if l.cfg.LogPath != "" {
		_ = os.MkdirAll(l.cfg.LogPath, 0o755)
		nivel := l.cfg.LogNivel
		if nivel == "" {
			nivel = "4"
		}
		preCfg := "[Principal]\nLogNivel=" + nivel + "\nLogPath=" + l.cfg.LogPath + "\n"
		_ = os.WriteFile(cfgPath, []byte(preCfg), 0o644)
	}

	cArqConfig := C.CString(cfgPath)
	cChaveCrypt := C.CString("")
	defer C.free(unsafe.Pointer(cArqConfig))
	defer C.free(unsafe.Pointer(cChaveCrypt))

	var h C.LibHandle
	if ret := C.NFSE_Inicializar(&h, cArqConfig, cChaveCrypt); ret != 0 {
		return Result{}, fmt.Errorf("NFSE_Inicializar falhou: código %d", int(ret))
	}
	defer C.NFSE_Finalizar(h)

	if err := applyConfig(h, t, sessionDir, l.cfg.SchemasPath); err != nil {
		return Result{}, err
	}
	return fn(h)
}

// applyConfig grava as chaves de configuração na instância: SSL para Linux,
// certificado A1 (se informado) e as chaves adicionais do tenant.
func applyConfig(h C.LibHandle, t TenantConfig, savePath, schemasPath string) error {
	kvs := []ConfigKV{
		{"DFe", "SSLCryptLib", sslCryptOpenSSL},
		{"DFe", "SSLHttpLib", sslHttpOpenSSL},
		{"DFe", "SSLXmlSignLib", sslXmlLibXml2},
		// Fuso do documento pela UF do emitente, não pelo relógio do servidor.
		// O default da lib (tzSistema) carimba o fuso do PROCESSO — e o container
		// roda em UTC, então todo dhEmi saía com +00:00: um CT-e emitido às 08:00
		// em São Paulo virava 08:00Z, três horas adiantado. Com tzPCN a lib usa a
		// tabela oficial por UF (AC -05, AM/RR/RO/MT/MS -04, demais -03), com
		// horário de verão. Ver GetUTCUF em ACBrUtil.DateTime.pas.
		{"DFe", "TimeZone.Modo", timeZonePorUF},
		// Caminho gravável para os XMLs (o default cai no dir do executável,
		// não-gravável). [NFSe] PathSalvar.
		{"NFSe", "PathSalvar", savePath},
		// DANFSE: diretório gravável para o PDF gerado pelo FortesReport. O
		// SalvarPDF devolve base64, mas a lib também grava no PathPDF.
		{"DANFSe", "PathPDF", savePath},
	}
	// Schemas XSD por provedor (validação). Interno — o cliente não informa isso.
	if schemasPath != "" {
		kvs = append(kvs, ConfigKV{"NFSe", "PathSchemas", schemasPath})
	}
	if t.PFXBase64 != "" {
		kvs = append(kvs,
			ConfigKV{"DFe", "DadosPFX", t.PFXBase64},
			ConfigKV{"DFe", "Senha", t.SenhaPFX},
		)
	}
	kvs = append(kvs, t.Config...)

	for _, kv := range kvs {
		if code := configGravarValor(h, kv); code != 0 {
			return fmt.Errorf("ConfigGravarValor [%s]%s falhou: %d", kv.Section, kv.Key, code)
		}
	}
	return nil
}

func configGravarValor(h C.LibHandle, kv ConfigKV) int {
	cs := C.CString(kv.Section)
	ck := C.CString(kv.Key)
	cv := C.CString(kv.Value)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ck))
	defer C.free(unsafe.Pointer(cv))
	return int(C.NFSE_ConfigGravarValor(h, cs, ck, cv))
}

func configCarregarINI(h C.LibHandle, ini string) (int, string) {
	cIni, freeIni := cstrFree(ini)
	defer freeIni()
	code := int(C.NFSE_CarregarINI(h, cIni))
	if code != 0 {
		return code, ultimoRetorno(h, 0)
	}
	return 0, ""
}

// callBuffer aplica a convenção de buffer da ACBrLib (mesma do SDK oficial):
// chama com um buffer de bufferLen; se esTamanho > bufferLen, a resposta é
// maior e é relida via NFSE_UltimoRetorno. Em caso de erro sem conteúdo no
// buffer, busca a mensagem no último retorno.
func callBuffer(h C.LibHandle, call func(buf *C.char, size *C.int) C.int) (int, string) {
	buf := make([]byte, bufferLen)
	size := C.int(bufferLen)
	ret := int(call((*C.char)(unsafe.Pointer(&buf[0])), &size))

	var resp string
	if int(size) > bufferLen {
		resp = ultimoRetorno(h, int(size))
	} else {
		resp = cstr(buf)
	}
	if ret != 0 && resp == "" {
		resp = ultimoRetorno(h, 0)
	}
	return ret, resp
}

// ultimoRetorno lê NFSE_UltimoRetorno com dimensionamento (hint do tamanho).
func ultimoRetorno(h C.LibHandle, hint int) string {
	n := hint
	if n < bufferLen {
		n = bufferLen
	}
	buf := make([]byte, n+1)
	size := C.int(n)
	C.NFSE_UltimoRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	if int(size) > n { // resposta maior que o hint: realoca uma vez
		n = int(size)
		buf = make([]byte, n+1)
		size = C.int(n)
		C.NFSE_UltimoRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	}
	return cstr(buf)
}

// cstrFree aloca uma C string e devolve o ponteiro + a função que a libera (use
// com `defer`). Encapsula o par C.CString / C.free(unsafe.Pointer(...)). Em código
// NOVO do binding, prefira-o ao free manual — esquecer um free vaza memória nativa.
// Ver docs/BINDING-CGO.md.
func cstrFree(s string) (*C.char, func()) {
	c := C.CString(s)
	return c, func() { C.free(unsafe.Pointer(c)) }
}

// cstr lê a string C (até o NUL) de um buffer, normalizando o encoding para
// UTF-8. As respostas estruturadas vêm em UTF-8, mas mensagens de exceção do
// ACBr podem vir em Latin-1/CP1252 (literais Pascal). Se não for UTF-8 válido,
// trata como Latin-1 (acentos PT-BR são idênticos nas duas codificações).
func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(rune(c)) // byte Latin-1 -> rune (code point idêntico)
	}
	return sb.String()
}

// tenantSlug reduz o CNPJ a dígitos para compor um nome de arquivo seguro.
func tenantSlug(cnpj string) string {
	var b strings.Builder
	for _, r := range cnpj {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}
