//go:build acbrlib

// Binding cgo da ACBrLibBoleto (libacbrboleto64, variante MT). NÃO é fiscal:
// gera o boleto/PDF a partir de um INI combinado ([Cedente]/[Conta]/[Banco]/
// [TituloN]). Sessão própria (sem SSL/cert/schemas dos DFe): Inicializar →
// config de saída (FCFortes: PDF gravável, preview off) → IncluirTitulos →
// SalvarPDF → Finalizar. Fixa a thread (lib não-reentrante por handle).
package acbr

/*
#include "acbrlib.h"
*/
import "C"

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"github.com/4devsmart/wrapper-api/internal/platform/config"
)

type boletoLib struct {
	cfg config.ACBr
}

func newBoletoLib(cfg config.ACBr) *boletoLib { return &boletoLib{cfg: cfg} }

func (l *boletoLib) Backend() string { return "cgo" }
func (l *boletoLib) Close() error    { return nil }

func (l *boletoLib) Version() (string, error) {
	res, err := l.withSession(func(h C.LibHandle, _ string) (Result, error) {
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.Boleto_Versao(h, b, sz) })
		return Result{Codigo: code, Resposta: resp}, nil
	})
	if err != nil {
		return "", err
	}
	if res.Codigo != 0 {
		return "", fmt.Errorf("Boleto_Versao retornou %d: %s", res.Codigo, res.Resposta)
	}
	return res.Resposta, nil
}

// GerarPDF inclui os títulos do INI (config+títulos) e devolve o PDF do boleto.
// tpSaida vazio em IncluirTitulos: só carrega; o PDF é gerado por SalvarPDF
// (base64). Geração offline, não fala com o banco.
func (l *boletoLib) GerarPDF(_ TenantConfig, ini string) (Result, error) {
	return l.withSession(func(h C.LibHandle, _ string) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		cTp := C.CString("")
		defer C.free(unsafe.Pointer(cTp))
		if code := int(C.Boleto_IncluirTitulos(h, cIni, cTp)); code != 0 {
			return Result{Codigo: code, Resposta: l.ultRet(h, 0)}, fmt.Errorf("Boleto_IncluirTitulos falhou: %d", code)
		}
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int { return C.Boleto_SalvarPDF(h, b, sz) })
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			res.PDF = pdfDecodificado(resp)
		}
		return res, nil
	})
}

// GerarRemessa inclui os títulos e gera o arquivo de remessa CNAB. O
// GerarRemessaStream devolve o arquivo em base64 → decodificamos para o texto
// CNAB em Result.Resposta.
func (l *boletoLib) GerarRemessa(_ TenantConfig, ini string, numArquivo int) (Result, error) {
	return l.withSession(func(h C.LibHandle, _ string) (Result, error) {
		cIni := C.CString(ini)
		defer C.free(unsafe.Pointer(cIni))
		cTp := C.CString("")
		defer C.free(unsafe.Pointer(cTp))
		if code := int(C.Boleto_IncluirTitulos(h, cIni, cTp)); code != 0 {
			return Result{Codigo: code, Resposta: l.ultRet(h, 0)}, fmt.Errorf("Boleto_IncluirTitulos falhou: %d", code)
		}
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.Boleto_GerarRemessaStream(h, C.int(numArquivo), b, sz)
		})
		res := Result{Codigo: code, Resposta: resp}
		if code == 0 {
			if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(resp)); err == nil {
				res.Resposta = string(dec)
			}
		}
		return res, nil
	})
}

// LerRetorno configura o banco (configINI) e processa o arquivo de retorno CNAB
// processa um arquivo de retorno CNAB e devolve o retorno parseado em JSON
// estruturado. O caminho do JSON é: TipoResposta=2 ([Principal]) + ObterRetorno
// (que lê de ARQUIVO e gera no formato configurado). LerRetornoStream sempre
// devolveria INI (ignora TipoResposta), por isso gravamos o conteúdo num arquivo
// temporário da sessão. Result.Resposta = JSON (árvore Retorno -> CEDENTE/BANCO/
// CONTA/TITULOn). Ver skill acbr-especialista/referencia/boleto-retorno.md.
func (l *boletoLib) LerRetorno(_ TenantConfig, configINI, retornoConteudo string) (Result, error) {
	return l.withSession(func(h C.LibHandle, dir string) (Result, error) {
		cCfg := C.CString(configINI)
		defer C.free(unsafe.Pointer(cCfg))
		if code := int(C.Boleto_ConfigurarDados(h, cCfg)); code != 0 {
			return Result{Codigo: code, Resposta: l.ultRet(h, 0)}, fmt.Errorf("Boleto_ConfigurarDados falhou: %d", code)
		}
		l.cfgVal(h, "Principal", "TipoResposta", "2") // 2 = JSON
		nome := "retorno.ret"
		if err := os.WriteFile(filepath.Join(dir, nome), []byte(retornoConteudo), 0o600); err != nil {
			return Result{}, fmt.Errorf("gravando retorno temporário: %w", err)
		}
		cDir, cNome := C.CString(dir), C.CString(nome)
		defer C.free(unsafe.Pointer(cDir))
		defer C.free(unsafe.Pointer(cNome))
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.Boleto_ObterRetorno(h, cDir, cNome, b, sz)
		})
		return Result{Codigo: code, Resposta: resp}, nil // resp já é JSON
	})
}

// Registrar registra o(s) boleto(s) ONLINE via API do banco. operacao:
// 0=incluir, 1=alterar, 2=baixar, 3/4=consultar. crt/key = certificado mTLS (ou
// nil). Erro de credencial/conexão volta como Codigo!=0 + msg (não derruba a
// API). NOTA: validação real exige credenciais de sandbox do banco.
func (l *boletoLib) Registrar(_ TenantConfig, op BoletoOnline) (Result, error) {
	return l.withSession(func(h C.LibHandle, dir string) (Result, error) {
		l.setCert(h, dir, op.CertCRT, op.CertKEY)
		cIni := C.CString(op.INI)
		defer C.free(unsafe.Pointer(cIni))
		cTp := C.CString("")
		defer C.free(unsafe.Pointer(cTp))
		if code := int(C.Boleto_IncluirTitulos(h, cIni, cTp)); code != 0 {
			return Result{Codigo: code, Resposta: l.ultRet(h, 0)}, fmt.Errorf("Boleto_IncluirTitulos falhou: %d", code)
		}
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.Boleto_EnviarBoleto(h, C.int(op.Operacao), b, sz)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// Consultar consulta títulos por período na API do banco. configINI =
// config do banco (com credenciais WS); filtroINI = [BoletoConsulta] com o
// período/filtros.
func (l *boletoLib) ConsultarTitulos(_ TenantConfig, op BoletoOnline) (Result, error) {
	return l.withSession(func(h C.LibHandle, dir string) (Result, error) {
		l.setCert(h, dir, op.CertCRT, op.CertKEY)
		cCfg := C.CString(op.ConfigINI)
		defer C.free(unsafe.Pointer(cCfg))
		if code := int(C.Boleto_ConfigurarDados(h, cCfg)); code != 0 {
			return Result{Codigo: code, Resposta: l.ultRet(h, 0)}, fmt.Errorf("Boleto_ConfigurarDados falhou: %d", code)
		}
		cFil := C.CString(op.FiltroINI)
		defer C.free(unsafe.Pointer(cFil))
		code, resp := l.callBuf(h, func(b *C.char, sz *C.int) C.int {
			return C.Boleto_ConsultarTitulosPorPeriodo(h, cFil, b, sz)
		})
		return Result{Codigo: code, Resposta: resp}, nil
	})
}

// setCert grava o certificado mTLS (CRT/KEY) na sessão e aponta a config do WS.
func (l *boletoLib) setCert(h C.LibHandle, dir string, crt, key []byte) {
	if len(crt) > 0 {
		p := filepath.Join(dir, "cert.crt")
		if os.WriteFile(p, crt, 0o600) == nil {
			l.cfgVal(h, "BoletoWebSevice", "ArquivoCRT", p)
			l.cfgVal(h, "BoletoWebSevice", "UseCertificateHTTP", "1")
		}
	}
	if len(key) > 0 {
		p := filepath.Join(dir, "cert.key")
		if os.WriteFile(p, key, 0o600) == nil {
			l.cfgVal(h, "BoletoWebSevice", "ArquivoKEY", p)
		}
	}
}

// withSession inicializa o handle MT, configura a saída do PDF (FCFortes) e
// finaliza. Fixa a thread durante toda a sequência.
func (l *boletoLib) withSession(fn func(h C.LibHandle, dir string) (Result, error)) (Result, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	base := l.cfg.IniBasePath
	if base == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return Result{}, fmt.Errorf("criando diretório base: %w", err)
	}
	sessionDir, err := os.MkdirTemp(base, "boleto-")
	if err != nil {
		return Result{}, fmt.Errorf("criando diretório de sessão: %w", err)
	}
	defer os.RemoveAll(sessionDir)
	cfgPath := filepath.Join(sessionDir, "acbrlibboleto.ini")

	cArq := C.CString(cfgPath)
	cKey := C.CString("")
	defer C.free(unsafe.Pointer(cArq))
	defer C.free(unsafe.Pointer(cKey))

	var h C.LibHandle
	if ret := C.Boleto_Inicializar(&h, cArq, cKey); ret != 0 {
		return Result{}, fmt.Errorf("Boleto_Inicializar falhou: código %d", int(ret))
	}
	defer C.Boleto_Finalizar(h)

	// Saída do PDF (FortesReport): arquivo gravável + GUI off (Xvfb não interage).
	l.cfgVal(h, "BoletoBancoFCFortesConfig", "NomeArquivo", filepath.Join(sessionDir, "boleto.pdf"))
	l.cfgVal(h, "BoletoBancoFCFortesConfig", "MostrarPreview", "0")
	l.cfgVal(h, "BoletoBancoFCFortesConfig", "MostrarProgresso", "0")
	l.cfgVal(h, "BoletoBancoFCFortesConfig", "MostrarSetup", "0")
	return fn(h, sessionDir)
}

func (l *boletoLib) cfgVal(h C.LibHandle, sec, key, val string) {
	cs, ck, cv := C.CString(sec), C.CString(key), C.CString(val)
	C.Boleto_ConfigGravarValor(h, cs, ck, cv)
	C.free(unsafe.Pointer(cs))
	C.free(unsafe.Pointer(ck))
	C.free(unsafe.Pointer(cv))
}

// callBuf/ultRet: convenção de buffer da ACBrLib, usando Boleto_UltimoRetorno
// (NÃO a do NFSe: é outra .so).
func (l *boletoLib) callBuf(h C.LibHandle, call func(buf *C.char, size *C.int) C.int) (int, string) {
	buf := make([]byte, bufferLen)
	size := C.int(bufferLen)
	ret := int(call((*C.char)(unsafe.Pointer(&buf[0])), &size))
	var resp string
	if int(size) > bufferLen {
		resp = l.ultRet(h, int(size))
	} else {
		resp = cstr(buf)
	}
	if ret != 0 && resp == "" {
		resp = l.ultRet(h, 0)
	}
	return ret, resp
}

func (l *boletoLib) ultRet(h C.LibHandle, hint int) string {
	n := hint
	if n < bufferLen {
		n = bufferLen
	}
	buf := make([]byte, n+1)
	size := C.int(n)
	C.Boleto_UltimoRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	if int(size) > n {
		n = int(size)
		buf = make([]byte, n+1)
		size = C.int(n)
		C.Boleto_UltimoRetorno(h, (*C.char)(unsafe.Pointer(&buf[0])), &size)
	}
	return cstr(buf)
}
