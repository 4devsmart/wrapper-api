/* Declarações C das ACBrLib (variante MultiThread). Compartilhado pelos
 * arquivos cgo (acbr_cgo*.go) via #include, para que o tipo LibHandle seja o
 * mesmo entre eles e cada serviço fique em seu próprio .go.
 *
 * Assinaturas conferidas nos imports MT oficiais do ACBr
 * (Projetos/ACBrLib/Fontes/{NFSe,CTe,MDFe}/*StaticImportMT.pas).
 * Boolean do Pascal = 1 byte → unsigned char no C ABI. */
#ifndef WRAPPER_ACBRLIB_H
#define WRAPPER_ACBRLIB_H

#include <stdlib.h>

typedef void* LibHandle;

/* ----- NFSe (libacbrnfse64) ----- */
extern int NFSE_Inicializar(LibHandle* handle, const char* eArqConfig, const char* eChaveCrypt);
extern int NFSE_Finalizar(LibHandle handle);
extern int NFSE_Versao(LibHandle handle, char* sVersao, int* esTamanho);
extern int NFSE_UltimoRetorno(LibHandle handle, char* sMensagem, int* esTamanho);
extern int NFSE_ConfigGravarValor(LibHandle handle, const char* eSessao, const char* eChave, const char* eValor);
extern int NFSE_CarregarINI(LibHandle handle, const char* eArquivoOuINI);
extern int NFSE_CarregarXML(LibHandle handle, const char* eArquivoOuXML);
extern int NFSE_ObterIni(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int NFSE_ObterXml(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int NFSE_ObterXmlRps(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int NFSE_Emitir(LibHandle handle, const char* aLote, int aModoEnvio, unsigned char aImprimir, char* sResposta, int* esTamanho);
extern int NFSE_Cancelar(LibHandle handle, const char* eInfCancelamento, char* sResposta, int* esTamanho);
extern int NFSE_SubstituirNFSe(LibHandle handle, const char* aNumeroNFSe, const char* aSerieNFSe, const char* aCodigoCancelamento, const char* aMotivoCancelamento, const char* aNumeroLote, const char* aCodigoVerificacao, char* sResposta, int* esTamanho);
extern int NFSE_SalvarPDF(LibHandle handle, char* sResposta, int* esTamanho);
extern int NFSE_ConsultarNFSePorChave(LibHandle handle, const char* aChaveNFSe, char* sResposta, int* esTamanho);
/* Consulta a NFS-e gerada a partir de uma DPS, pela chave DELA (o ADN atende
 * GET /dps/{chave} — ver PadraoNacional.Provider.pas:503). É o caminho de
 * recuperação quando a transmissão de uma DPS teve desfecho desconhecido.
 * Assinatura conferida em ACBrLibNFSeMT.pas:198 e ACBrNFSeMT.h:83. */
extern int NFSE_ConsultarDPSPorChave(LibHandle handle, const char* aChaveDPS, char* sResposta, int* esTamanho);
extern int NFSE_ObterDANFSE(LibHandle handle, const char* aChaveNFSe, char* sResposta, int* esTamanho);
/* Distribuição DF-e do ADN (Padrão Nacional): consulta por NSU. O CNPJ vem da
 * config do componente, não do parâmetro. */
extern int NFSE_ConsultarDFe(LibHandle handle, int aNSU, char* sResposta, int* esTamanho);
/* Consultas pontuais (sem TDateTime). As por período/serviço ficam p/ outra passada. */
extern int NFSE_ConsultarNFSePorNumero(LibHandle handle, const char* aNumero, int aPagina, char* sResposta, int* esTamanho);
extern int NFSE_ConsultarNFSePorFaixa(LibHandle handle, const char* aNumeroInicial, const char* aNumeroFinal, int aPagina, char* sResposta, int* esTamanho);
extern int NFSE_ConsultarNFSePorRps(LibHandle handle, const char* aNumeroRps, const char* aSerie, const char* aTipo, const char* aCodigoVerificacao, char* sResposta, int* esTamanho);
extern int NFSE_ConsultarSituacao(LibHandle handle, const char* aProtocolo, const char* aNumLote, char* sResposta, int* esTamanho);
extern int NFSE_ConsultarLoteRps(LibHandle handle, const char* aProtocolo, const char* aNumLote, char* sResposta, int* esTamanho);

/* ----- CT-e (libacbrcte64) ----- */
extern int CTE_Inicializar(LibHandle* handle, const char* eArqConfig, const char* eChaveCrypt);
extern int CTE_Finalizar(LibHandle handle);
extern int CTE_Versao(LibHandle handle, char* sVersao, int* esTamanho);
extern int CTE_UltimoRetorno(LibHandle handle, char* sMensagem, int* esTamanho);
extern int CTE_ConfigGravarValor(LibHandle handle, const char* eSessao, const char* eChave, const char* eValor);
extern int CTE_CarregarINI(LibHandle handle, const char* eArquivoOuINI);
extern int CTE_CarregarXML(LibHandle handle, const char* eArquivoOuXML);
extern int CTE_CarregarEventoINI(LibHandle handle, const char* eArquivoOuINI);
extern int CTE_ObterIni(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int CTE_ObterXml(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
/* ATENÇÃO: aSincrono existe e é OBRIGATÓRIO. O ACBrLibCTeStaticImportMT.pas
 * oficial o OMITE — está desatualizado em relação ao ACBrLibCTeMT.pas que ele
 * espelha. A ABI correta está confirmada em ACBrLibCTeMT.pas:122 e nos imports
 * MT de PHP (ACBrCTeMT.h), C# (ACBrCTe.Delegates.cs) e VB6. Chamar sem ele
 * desloca os argumentos de registrador no SysV AMD64: a lib passa a escrever a
 * resposta no ponteiro de tamanho e a ler o tamanho de um registrador sujo. */
extern int CTE_Enviar(LibHandle handle, int aLote, unsigned char aImprimir, unsigned char aSincrono, char* sResposta, int* esTamanho);
extern int CTE_ValidarRegrasdeNegocios(LibHandle handle, char* sResposta, int* esTamanho);
extern int CTE_Consultar(LibHandle handle, const char* eChaveOuCTe, char* sResposta, int* esTamanho);
extern int CTE_EnviarEvento(LibHandle handle, int idLote, char* sResposta, int* esTamanho);
extern int CTE_SalvarPDF(LibHandle handle, char* sResposta, int* esTamanho);
extern int CTE_SalvarEventoPDF(LibHandle handle, const char* eArquivoXmlCTe, const char* eArquivoXmlEvento, char* sResposta, int* esTamanho);
/* Distribuição DF-e (CT-e exige AcUFAutor = cUF do autor). */
extern int CTE_DistribuicaoDFePorUltNSU(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* eultNSU, char* sResposta, int* esTamanho);
extern int CTE_DistribuicaoDFePorNSU(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* eNSU, char* sResposta, int* esTamanho);
extern int CTE_DistribuicaoDFePorChave(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* echCTe, char* sResposta, int* esTamanho);
/* Status/consultas. nIE: boolean Pascal = 1 byte → unsigned char. */
extern int CTE_StatusServico(LibHandle handle, char* sResposta, int* esTamanho);
extern int CTE_ConsultarRecibo(LibHandle handle, const char* ARecibo, char* sResposta, int* esTamanho);
extern int CTE_ConsultaCadastro(LibHandle handle, const char* cUF, const char* nDocumento, unsigned char nIE, char* sResposta, int* esTamanho);

/* ----- MDF-e (libacbrmdfe64) ----- */
extern int MDFE_Inicializar(LibHandle* handle, const char* eArqConfig, const char* eChaveCrypt);
extern int MDFE_Finalizar(LibHandle handle);
extern int MDFE_Versao(LibHandle handle, char* sVersao, int* esTamanho);
extern int MDFE_UltimoRetorno(LibHandle handle, char* sMensagem, int* esTamanho);
extern int MDFE_ConfigGravarValor(LibHandle handle, const char* eSessao, const char* eChave, const char* eValor);
extern int MDFE_CarregarINI(LibHandle handle, const char* eArquivoOuINI);
extern int MDFE_CarregarXML(LibHandle handle, const char* eArquivoOuXML);
extern int MDFE_CarregarEventoINI(LibHandle handle, const char* eArquivoOuINI);
extern int MDFE_ObterIni(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int MDFE_ObterXml(LibHandle handle, int AIndex, char* sResposta, int* esTamanho);
extern int MDFE_Enviar(LibHandle handle, int aLote, unsigned char aImprimir, unsigned char aSincrono, char* sResposta, int* esTamanho);
extern int MDFE_ValidarRegrasdeNegocios(LibHandle handle, char* sResposta, int* esTamanho);
extern int MDFE_Consultar(LibHandle handle, const char* eChaveOuMDFe, char* sResposta, int* esTamanho);
extern int MDFE_EnviarEvento(LibHandle handle, int idLote, char* sResposta, int* esTamanho);
extern int MDFE_SalvarPDF(LibHandle handle, char* sResposta, int* esTamanho);
extern int MDFE_SalvarEventoPDF(LibHandle handle, const char* eArquivoXmlMDFe, const char* eArquivoXmlEvento, char* sResposta, int* esTamanho);
/* Distribuição DF-e (MDF-e NÃO recebe AcUFAutor — diferente do CT-e/NFe). */
extern int MDFE_DistribuicaoDFePorUltNSU(LibHandle handle, const char* eCNPJCPF, const char* eultNSU, char* sResposta, int* esTamanho);
extern int MDFE_DistribuicaoDFePorNSU(LibHandle handle, const char* eCNPJCPF, const char* eNSU, char* sResposta, int* esTamanho);
extern int MDFE_DistribuicaoDFePorChave(LibHandle handle, const char* eCNPJCPF, const char* echMDFe, char* sResposta, int* esTamanho);
/* Status/consultas. MDFE_ConsultaMDFeNaoEnc confirmado em DUAS fontes oficiais
 * independentes — ACBrLibMDFeMT.pas:133 e o header C do demo PHP
 * (ACBrMDFeMT.h:85) — com a assinatura abaixo. Não precisa de opt-in. */
extern int MDFE_StatusServico(LibHandle handle, char* sResposta, int* esTamanho);
extern int MDFE_ConsultarRecibo(LibHandle handle, const char* ARecibo, char* sResposta, int* esTamanho);
extern int MDFE_ConsultaMDFeNaoEnc(LibHandle handle, const char* nCNPJ, char* sResposta, int* esTamanho);

/* ----- NF-e (libacbrnfe64) — APENAS Distribuição DF-e + Manifestação ----- */
/* (sem emissão: o escopo do projeto é receber NF-e contra o CNPJ e manifestar). */
extern int NFE_Inicializar(LibHandle* handle, const char* eArqConfig, const char* eChaveCrypt);
extern int NFE_Finalizar(LibHandle handle);
extern int NFE_Versao(LibHandle handle, char* sVersao, int* esTamanho);
extern int NFE_UltimoRetorno(LibHandle handle, char* sMensagem, int* esTamanho);
extern int NFE_ConfigGravarValor(LibHandle handle, const char* eSessao, const char* eChave, const char* eValor);
extern int NFE_CarregarEventoINI(LibHandle handle, const char* eArquivoOuINI);
extern int NFE_EnviarEvento(LibHandle handle, int idLote, char* sResposta, int* esTamanho);
extern int NFE_DistribuicaoDFePorUltNSU(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* eultNSU, char* sResposta, int* esTamanho);
extern int NFE_DistribuicaoDFePorNSU(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* eNSU, char* sResposta, int* esTamanho);
extern int NFE_DistribuicaoDFePorChave(LibHandle handle, int AcUFAutor, const char* eCNPJCPF, const char* echNFe, char* sResposta, int* esTamanho);

/* ----- Boleto (libacbrboleto64) — não-fiscal: boleto/PDF + CNAB ----- */
extern int Boleto_Inicializar(LibHandle* handle, const char* eArqConfig, const char* eChaveCrypt);
extern int Boleto_Finalizar(LibHandle handle);
extern int Boleto_Versao(LibHandle handle, char* sVersao, int* esTamanho);
extern int Boleto_UltimoRetorno(LibHandle handle, char* sMensagem, int* esTamanho);
extern int Boleto_ConfigGravarValor(LibHandle handle, const char* eSessao, const char* eChave, const char* eValor);
extern int Boleto_ConfigurarDados(LibHandle handle, const char* eArquivoIni);
extern int Boleto_IncluirTitulos(LibHandle handle, const char* eArquivoIni, const char* eTpSaida);
extern int Boleto_LimparLista(LibHandle handle);
extern int Boleto_TotalTitulosLista(LibHandle handle);
extern int Boleto_GerarPDF(LibHandle handle);
extern int Boleto_SalvarPDF(LibHandle handle, char* sResposta, int* esTamanho);
extern int Boleto_GerarRemessaStream(LibHandle handle, int eNumArquivo, char* sResposta, int* esTamanho);
extern int Boleto_LerRetornoStream(LibHandle handle, const char* ARetornoBase64, char* sResposta, int* esTamanho);
extern int Boleto_ObterRetorno(LibHandle handle, const char* eDir, const char* eNomeArq, char* sResposta, int* esTamanho);
extern int Boleto_GerarToken(LibHandle handle, char* sResposta, int* esTamanho);
extern int Boleto_EnviarBoleto(LibHandle handle, int eCodigoOperacao, char* sResposta, int* esTamanho);
extern int Boleto_ConsultarTitulosPorPeriodo(LibHandle handle, const char* eArquivoIni, char* sResposta, int* esTamanho);

#endif /* WRAPPER_ACBRLIB_H */
