#!/usr/bin/env python3
"""Regenera internal/tabelas/municipios_provedor.tsv a partir do fonte do ACBr.

A tabela mapeia código IBGE -> (provedor de NFS-e, versão do layout) e sai do
ACBrNFSeXServicos.ini, a MESMA tabela que é compilada dentro da `.so`. Ela
precisa andar em lockstep com a revisão pinada em ACBR_REV: município que muda
de provedor no fonte e não muda aqui vira roteamento errado, silencioso.

Este script NÃO gera provedor_familia.tsv. A família sai da herança de classes
dos provedores, e a classificação carrega julgamento que nenhuma regra mecânica
reproduz: o Betha declara ABRASFv1, ABRASFv2 e uma APIPropria descendente do
Padrão Nacional e está classificado como abrasf_v1_v2, enquanto o Fiorilli, com
ABRASFv2 e a mesma APIPropria, está como proprio_abrasf. Gerar isso por regra
reclassificaria provedores em produção sem ninguém olhar. Em vez disso o script
RECUSA emitir a tabela quando aparece provedor sem família, que é o que força a
decisão humana no lugar certo.

Uso:  make acbr-tabelas
      python3 scripts/gerar-tabelas-nfse.py [caminho-do-ACBrNFSeXServicos.ini]
"""
import pathlib
import re
import sys

RAIZ = pathlib.Path(__file__).resolve().parent.parent
INI_PADRAO = RAIZ / "acbr-source/acbr/Fontes/ACBrDFe/ACBrNFSeX/ACBrNFSeXServicos.ini"
DESTINO = RAIZ / "internal/tabelas/municipios_provedor.tsv"
FAMILIAS = RAIZ / "internal/tabelas/provedor_familia.tsv"
MUNICIPIOS = RAIZ / "internal/tabelas/municipios.tsv"
PROVEDORES_PAS = RAIZ / "acbr-source/acbr/Fontes/ACBrDFe/ACBrNFSeX/Provedores"

CODIGO_IBGE = re.compile(r"\d{7}")


def ler_ini(caminho):
    """(seção -> {chave: valor}), na ordem do arquivo.

    O .ini vem em latin-1 e com CRLF. Chave repetida dentro da mesma seção fica
    com o PRIMEIRO valor, que é como o TIniFile do ACBr resolve o empate.
    """
    texto = caminho.read_text(encoding="latin-1").replace("\r\n", "\n")
    secoes, atual = {}, None
    for linha in texto.split("\n"):
        l = linha.strip()
        if l.startswith("[") and l.endswith("]"):
            atual = l[1:-1]
            secoes.setdefault(atual, {})
        elif atual and "=" in l and not l.startswith(";"):
            chave, _, valor = l.partition("=")
            secoes[atual].setdefault(chave.strip(), valor.strip())
    return secoes


def tabela(secoes):
    """[(codigo, provedor, versao)] dos municípios COM provedor configurado."""
    linhas = []
    for nome, campos in secoes.items():
        if not CODIGO_IBGE.fullmatch(nome):
            continue
        provedor = campos.get("Provedor", "")
        if not provedor:
            continue
        linhas.append((nome, provedor, campos.get("Versao", "")))
    return sorted(linhas)


def coluna(caminho, indice=0):
    fora = set()
    for l in caminho.read_text(encoding="utf-8").split("\n"):
        if not l.strip():
            continue
        campos = l.split("\t")
        if len(campos) > indice and campos[indice]:
            fora.add(campos[indice])
    return fora


def familia_sugerida(provedor):
    """Ancestrais declarados no .pas do provedor, para ajudar a classificar."""
    pas = PROVEDORES_PAS / f"{provedor}.Provider.pas"
    if not pas.exists():
        return ""
    classes = re.findall(
        r"(TACBrNFSeProvider[A-Za-z0-9_]*)\s*=\s*class\s*\(\s*([A-Za-z0-9_]+)\s*\)",
        pas.read_text(encoding="latin-1"),
    )
    return ", ".join(f"{filha} < {mae}" for filha, mae in classes)


def main():
    ini = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else INI_PADRAO
    if not ini.exists():
        print(f"{ini} não está aqui. Rode 'make acbr-fonte' e tente de novo.")
        return 1

    linhas = tabela(ler_ini(ini))
    if not linhas:
        print(f"{ini} não rendeu nenhum município com provedor: formato mudou?")
        return 1

    # Gate 1: provedor sem família não entra. É a classe de quebra que o bump de
    # r47859 para r48100 trouxe (o provedor NFOnline estreou em São Bento/PB), e
    # que TestProvedoresNFSeTemFamilia só pegaria depois do arquivo já gravado.
    conhecidas = {p.lower() for p in coluna(FAMILIAS)}
    sem_familia = sorted({p for _, p, _ in linhas if p.lower() not in conhecidas})
    if sem_familia:
        print("provedor sem família em internal/tabelas/provedor_familia.tsv:")
        for p in sem_familia:
            quantos = sum(1 for _, prov, _ in linhas if prov == p)
            print(f"  {p} ({quantos} município(s))")
            if heranca := familia_sugerida(p):
                print(f"      herança no fonte: {heranca}")
        print()
        print("Classifique cada um em provedor_familia.tsv e rode de novo.")
        print("A família sai da classe ancestral: ABRASFv1 -> abrasf_v1,")
        print("ABRASFv2 -> abrasf_v2, Proprio -> proprio; mais de uma, decida.")
        return 1

    # Gate 2: código que não existe na base do IBGE viraria linha órfã, sem
    # nome nem UF na resposta da API.
    ibge = coluna(MUNICIPIOS)
    orfaos = sorted({c for c, _, _ in linhas if c not in ibge})
    if orfaos:
        print(f"códigos ausentes de municipios.tsv: {', '.join(orfaos)}")
        return 1

    antes = DESTINO.read_text(encoding="utf-8") if DESTINO.exists() else ""
    novo = "\n".join(f"{c}\t{p}\t{v}" for c, p, v in linhas) + "\n"
    DESTINO.write_text(novo, encoding="utf-8")

    velhas, novas = set(antes.split("\n")) - {""}, set(novo.split("\n")) - {""}
    print(f"{DESTINO.relative_to(RAIZ)}: {len(linhas)} municípios com provedor")
    if antes:
        print(f"  {len(novas - velhas)} linha(s) a mais, {len(velhas - novas)} a menos")
    return 0


if __name__ == "__main__":
    sys.exit(main())
