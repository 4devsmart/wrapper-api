#!/usr/bin/env python3
"""Gera os snapshots de chaves de INI que a ACBrLib aceita, do fonte do ACBr.

Cada `internal/{cte,mdfe,nfse}/testdata/lerini_chaves.tsv` é o conjunto
(seção, chave) que o leitor de INI da biblioteca consome. O lockstep compara
esse conjunto com o que os nossos construtores realmente escrevem, nas duas
direções. Até 2026-09 o snapshot era feito à mão, não tinha gerador, e estava
incompleto: faltavam as chaves de FALLBACK, aquelas que só aparecem como padrão
de um Read aninhado, como

    ReadString(sSecao, 'RegimeEspTrib', ReadString(sSecao, 'Regime', '0'))

onde 'Regime' também é aceito e o snapshot só guardava 'RegimeEspTrib'. Uma
chave assim, ausente do snapshot, faz o gate acusar chave morta que não é.

Uso:  make acbr-chaves
      python3 scripts/gerar-chaves-lerini.py [--conferir]

--conferir não escreve nada: compara com o que está versionado e falha se
divergir. É o que o CI roda para pegar snapshot desatualizado em relação ao
fonte pinado.
"""
import pathlib
import re
import sys

RAIZ = pathlib.Path(__file__).resolve().parent.parent
FONTE = RAIZ / "acbr-source/acbr/Fontes/ACBrDFe"

# documento -> (leitores no fonte, destino do snapshot)
#
# São dois arquivos por documento de DFe: o do documento e o dos EVENTOS, que
# vive em Base/Servicos e tem as seções [EVENTO], [DETEVENTO] e afins. A
# extração manual só olhava o primeiro, e por isso 91 chaves de evento do CT-e e
# 29 do MDF-e ficaram de fora do lockstep.
DOCUMENTOS = {
    "cte": (["ACBrCTe/Base/ACBrCTe.IniReader.pas",
             "ACBrCTe/Base/Servicos/ACBrCTe.EnvEvento.pas"],
            "internal/cte/testdata/lerini_chaves.tsv"),
    "mdfe": (["ACBrMDFe/Base/ACBrMDFe.IniReader.pas",
              "ACBrMDFe/Base/Servicos/ACBrMDFe.EnvEvento.pas"],
             "internal/mdfe/testdata/lerini_chaves.tsv"),
    # A NFS-e tinha a MESMA lacuna: o segundo arquivo é onde vivem as seções
    # dos pedidos que não são a nota ([CancelarNFSe], [Evento], as consultas),
    # e nenhuma delas era comparada. O INI de cancelamento que já enviávamos
    # nunca passou pelo gate, e o de evento também não passaria.
    #
    # As duas fontes têm uma seção de mesmo nome, [Evento]: no leitor da nota é
    # a atividade de evento (show, feira), no dos pedidos é o pedido de registro
    # de evento. Aqui elas se juntam, e o snapshot vira superconjunto das duas,
    # o que afrouxa o gate só nessa seção.
    "nfse": (["ACBrNFSeX/Base/Provedores/ACBrNFSeX.LerIni.pas",
              "ACBrNFSeX/Base/WebServices/ACBrNFSeXWebserviceBase.pas"],
             "internal/nfse/testdata/lerini_chaves.tsv"),
}

# sSecao := 'Nome';  ou  sSecao := 'Nome' + IntToStrZero(i, 3);
#
# O "+" no fim é o que distingue seção INDEXADA de seção com dígito no nome. A
# diferença importa: [det001] é det com índice, [ICMS60] e [toma4] são nomes. A
# extração manual adivinhava pelo dígito final e errava nos dois sentidos.
# Seção indexada sai marcada com "*" no snapshot.
# Pega o lado direito inteiro, até o ";", e dele TODOS os literais. O fonte
# escolhe o nome da seção de três formas, e a regra cobre as três:
#
#   sSecao := 'Servico';                                   -> {Servico}
#   sSecao := 'Itens' + IntToStrZero(i, 3);                -> {Itens*}
#   sSecao := IfThen(SectionExists('emiDocAnt'+...),
#                    'emiDocAnt', 'DocAnt') + IntToStrZero(I, 3);
#                                          -> {emiDocAnt*, DocAnt*}
#
# Na terceira, os dois nomes são aceitos pela biblioteca, então os dois entram.
# Capturar demais deixa o snapshot superconjunto, o que só torna o gate mais
# permissivo; capturar de menos acusaria chave morta que não é, que é o erro
# caro.
# Duas variáveis de seção no fonte: sSecao em quase tudo, e Secao em quatro
# leituras do CT-e. O padrão aceita as duas em vez de fixar um nome.
ATRIBUICAO = re.compile(r"\b(?:sSecao|Secao)\s*:=\s*([^;]*);", re.S)
CHAVE_VAR = re.compile(r"\bsKey\s*:=\s*([^;]*);", re.S)
LITERAL = re.compile(r"'([^']*)'")

# Braço de "case ... of": "teComprEntrega:" no início da linha, seguido ou não
# de "begin". Cada braço recomeça com a seção que valia no "case", não com a que
# o braço ANTERIOR deixou. Sem isto o rastreamento sequencial vaza de um braço
# para o outro e acusa chave morta que não é: o leitor de evento do CT-e põe
# 'dest'+índice num braço e lê dhEntrega noutro, onde sSecao ainda é
# 'EVENTO'+índice, e o nosso builder está certo em escrever [EVENTO001].
CASE = re.compile(r"^[ \t]*\bcase\b", re.M)
BRACO = re.compile(r"^[ \t]*([A-Za-z_]\w*)\s*:\s*(?:begin)?[ \t]*$", re.M)

# Read*(sSecao, 'chave'  ou  Read*('secao', 'chave'
#
# O findall varre o arquivo inteiro, então um Read aninhado dentro do argumento
# padrão de outro casa por conta própria: a chave de fallback entra sem
# tratamento especial. Era exatamente o que a extração manual perdia.
LEITURA = re.compile(r"\bRead[A-Za-z]+\(\s*(?:'([^']*)'|sSecao|Secao)\s*,\s*(?:'([^']+)'|(sKey))")

# sKey := 'cInfManu' + IntToStrZero(I,3);  e depois  ReadString(sSecao, sKey, '')
# Chave indexada por variável. Existe uma só em todo o fonte (o cInfManu do
# aéreo, no CT-e), e foi ela que denunciou que "nenhuma chave é concatenada"
# era medição minha, não fato.

CABECALHO = """\
# Chaves de INI aceitas pelo leitor de {doc} do ACBr (snapshot).
# NÃO editar à mão: gerado por scripts/gerar-chaves-lerini.py a partir de
# {origem}, na revisão fixada em ACBR_REV.
# Regenerar com `make acbr-chaves`; conferir com `make acbr-chaves-conferir`.
# Seção INDEXADA sai marcada com "*" (det* casa [det001], [det002]...).
# Seção sem "*" casa exata: [ICMS60] e [toma4] são NOMES, não índices.
# Chave sai literal, sempre: nenhuma é montada por concatenação no fonte.
# Formato: <secao>\\t<chave>
"""


def extrair(caminhos):
    """{(secao, chave)} lidos pelo leitor de INI, na ordem em que aparecem.

    A seção corrente vem da última atribuição a sSecao antes da leitura, o que
    espelha como o Pascal executa. Leitura com seção literal no lugar da
    variável usa a literal.
    """
    pares = set()
    for caminho in caminhos:
        pares |= extrair_de(caminho)
    return pares


def extrair_de(caminho):
    texto = caminho.read_text(encoding="latin-1")
    eventos = []
    for m in ATRIBUICAO.finditer(texto):
        eventos.append((m.start(), "secao", nomes(m.group(1))))
    for m in CHAVE_VAR.finditer(texto):
        eventos.append((m.start(), "skey", nomes(m.group(1))))
    for m in CASE.finditer(texto):
        eventos.append((m.start(), "case", None))
    for m in BRACO.finditer(texto):
        eventos.append((m.start(), "braco", None))
    for m in LEITURA.finditer(texto):
        eventos.append((m.start(), "chave", (m.group(1), m.group(2), m.group(3))))
    eventos.sort(key=lambda e: e[0])

    # Atribuições consecutivas a sSecao, sem leitura entre elas, são ALIAS: o
    # fonte faz
    #     sSecao := 'disp' + ...;
    #     if not SectionExists(sSecao) then sSecao := 'valePed' + ...;
    # e a biblioteca aceita os DOIS nomes. Guardar só o último perderia o
    # primeiro, e o gate acusaria chave morta em quem escrevesse [disp001].
    atuais, chaves_var, pares = [], [], set()
    no_case = []  # seção que valia ao entrar no "case" mais recente
    ultimo_foi_secao = False
    for _, tipo, valor in eventos:
        if tipo == "case":
            no_case = list(atuais)
            ultimo_foi_secao = False
            continue
        if tipo == "braco":
            if no_case:
                atuais = list(no_case)
            ultimo_foi_secao = False
            continue
        if tipo == "secao":
            if not ultimo_foi_secao:
                atuais = []
            atuais.extend(valor)
            ultimo_foi_secao = True
            continue
        ultimo_foi_secao = False
        if tipo == "skey":
            chaves_var = valor
            continue
        literal, chave, usou_skey = valor
        candidatas = chaves_var if usou_skey else ([chave] if chave else [])
        for secao in ([literal] if literal else atuais):
            for k in candidatas:
                if secao and k:
                    pares.add((secao, k))
    return pares


def nomes(expressao):
    """Nomes possíveis de uma atribuição de seção/chave, com "*" se indexado."""
    idx = "*" if "IntToStr" in expressao else ""
    # Literal só de dígitos é pedaço de índice montado à mão ('001' em
    # 'idDocAntPap' + IntToStrZero(I,3) + '001'), não nome de seção.
    return [lit + idx for lit in LITERAL.findall(expressao) if lit and not lit.isdigit()]


def render(doc, origem, pares):
    # Desempate pelo texto original: 'PropUF' e 'propUF' empatam em minúsculas,
    # e sem isto a ordem variava entre execuções, fazendo o --conferir acusar
    # divergência num arquivo de conteúdo idêntico.
    linhas = sorted(pares, key=lambda p: (p[0].lower(), p[1].lower(), p[0], p[1]))
    corpo = "".join(f"{s}\t{c}\n" for s, c in linhas)
    return CABECALHO.format(doc=doc.upper(), origem=origem) + corpo


def main():
    conferir = "--conferir" in sys.argv[1:]
    if not FONTE.exists():
        print(f"{FONTE} não está aqui. Rode 'make acbr-fonte' e tente de novo.")
        return 0 if conferir else 1

    divergiu = False
    for doc, (origens, destino) in DOCUMENTOS.items():
        pas = [FONTE / o for o in origens]
        for p in pas:
            if not p.exists():
                print(f"{doc}: leitor ausente em {p}")
                return 1
        pares = extrair(pas)
        if not pares:
            print(f"{doc}: nenhuma chave extraída, o formato mudou?")
            return 1
        novo = render(doc, ", ".join(origens), pares)
        alvo = RAIZ / destino
        atual = alvo.read_text(encoding="utf-8") if alvo.exists() else ""
        if conferir:
            if atual != novo:
                a = {l for l in atual.split("\n") if l and not l.startswith("#")}
                n = {l for l in novo.split("\n") if l and not l.startswith("#")}
                print(f"{destino} desatualizado: {len(n - a)} a mais, {len(a - n)} a menos")
                divergiu = True
            continue
        alvo.write_text(novo, encoding="utf-8")
        print(f"{destino}: {len(pares)} chaves")

    if conferir and divergiu:
        print()
        print("Rode 'make acbr-chaves' e reveja o diff: cada chave nova é uma decisão.")
        return 1
    if conferir:
        print("snapshots em dia com o fonte pinado")
    return 0


if __name__ == "__main__":
    sys.exit(main())
