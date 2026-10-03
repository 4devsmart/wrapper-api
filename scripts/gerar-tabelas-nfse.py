#!/usr/bin/env python3
"""Regenera internal/tabelas/municipios_provedor.tsv a partir do fonte do ACBr.

A tabela mapeia código IBGE -> (provedor de NFS-e, versão do layout, API
própria) e sai do ACBrNFSeXServicos.ini, a MESMA tabela que é compilada dentro da `.so`. Ela
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

A quarta coluna marca os municípios em que a lib instancia a classe APIPropria
do provedor. O gravador dessas classes descende do Padrão Nacional e gera DPS,
então é ela, e não a família do provedor, que decide o layout de entrada. A
regra é a de TGeralConfNFSe.SetCodigoMunicipio (ACBrNFSeXConfiguracoes.pas)
somada ao ramo `if APIPropria` do GetProvider (ACBrNFSeXProviderManager.pas).

Uso:  make acbr-tabelas
      python3 scripts/gerar-tabelas-nfse.py [--conferir] [caminho-do-ACBrNFSeXServicos.ini]

--conferir não escreve nada: compara com o que está versionado e falha se
divergir.
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
PROVIDER_MANAGER = (RAIZ / "acbr-source/acbr/Fontes/ACBrDFe/ACBrNFSeX/Base/Provedores"
                    / "ACBrNFSeXProviderManager.pas")

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


def provedores_com_api_propria(caminho):
    """Provedores cujo ramo no GetProvider começa por `if APIPropria then`.

    Só nesses o flag muda a classe instanciada. Nos outros, Params com
    APIPropria: não tem efeito na escolha do gravador.
    """
    texto = caminho.read_text(encoding="latin-1")
    return {m.lower() for m in re.findall(
        r"\bpro(\w+):\s*begin\s*if\s+APIPropria\s+then", texto)}


def secao(secoes, nome):
    """A seção pelo nome, sem diferenciar caixa, como o TIniFile do ACBr.

    Esperança do Sul declara Provedor=Abase e a seção é [ABase]; com busca
    exata o município ficava sem os Params do provedor e sem a marca de API
    própria, e a lib usa a API própria ali.
    """
    if nome in secoes:
        return secoes[nome]
    alvo = nome.lower()
    return next((v for k, v in secoes.items() if k.lower() == alvo), None)


def versao_resolvida(secoes, codigo, provedor):
    """A versão que a lib usa, como em TGeralConfNFSe.SetCodigoMunicipio.

    Com seção do provedor, a versão do município vence se existir, senão vale a
    do provedor (default 1.00). Sem seção do provedor, a do município (default
    1.00). "***" deixa a versão anterior da sessão, que daqui não se conhece:
    devolve None.
    """
    municipio = secoes[codigo].get("Versao", "")
    do_provedor_sec = secao(secoes, provedor)
    if do_provedor_sec is not None:
        do_provedor = do_provedor_sec.get("Versao", "1.00")
        if do_provedor and municipio:
            return None if municipio == "***" else municipio
        return do_provedor
    municipio = municipio or "1.00"
    return None if municipio == "***" else municipio


def api_propria(secoes, codigo, provedor):
    """O FAPIPropria de TGeralConfNFSe.SetCodigoMunicipio.

    Params do município, concatenado ao do provedor salvo quando o do município
    tem "*"; APIPropria: presente; e só vale nas versões 1.00 e 1.01. A string
    "APIPropria:" é procurada como a lib procura, por substring.
    """
    params = secoes[codigo].get("Params", "")
    if "*" not in params:
        params += (secao(secoes, provedor) or {}).get("Params", "")
    if "APIPropria:" not in params:
        return False
    versao = versao_resolvida(secoes, codigo, provedor)
    if versao is None:
        raise ValueError(f"{codigo}: Versao=*** com APIPropria, a lib decide em tempo de execução")
    return versao in ("1.00", "1.01")


def tabela(secoes, com_ramo):
    """[(codigo, provedor, versao, api_propria)] dos municípios COM provedor.

    api_propria só é marcado quando o provedor tem o ramo no GetProvider: é a
    condição para a lib instanciar a classe APIPropria, cujo gravador descende
    do Padrão Nacional e gera DPS.
    """
    linhas = []
    for nome, campos in secoes.items():
        if not CODIGO_IBGE.fullmatch(nome):
            continue
        provedor = campos.get("Provedor", "")
        if not provedor:
            continue
        propria = provedor.lower() in com_ramo and api_propria(secoes, nome, provedor)
        linhas.append((nome, provedor, campos.get("Versao", ""), "1" if propria else ""))
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
    conferir = "--conferir" in sys.argv[1:]
    args = [a for a in sys.argv[1:] if a != "--conferir"]
    ini = pathlib.Path(args[0]) if args else INI_PADRAO
    if not ini.exists():
        print(f"{ini} não está aqui. Rode 'make acbr-fonte' e tente de novo.")
        return 0 if conferir else 1

    if not PROVIDER_MANAGER.exists():
        print(f"{PROVIDER_MANAGER} não está aqui. Rode 'make acbr-fonte' e tente de novo.")
        return 1
    com_ramo = provedores_com_api_propria(PROVIDER_MANAGER)
    if not com_ramo:
        print(f"{PROVIDER_MANAGER} não rendeu nenhum ramo 'if APIPropria': formato mudou?")
        return 1
    try:
        linhas = tabela(ler_ini(ini), com_ramo)
    except ValueError as e:
        print(e)
        return 1
    if not linhas:
        print(f"{ini} não rendeu nenhum município com provedor: formato mudou?")
        return 1

    # Gate 1: provedor sem família não entra. É a classe de quebra que o bump de
    # r47859 para r48100 trouxe (o provedor NFOnline estreou em São Bento/PB), e
    # que TestProvedoresNFSeTemFamilia só pegaria depois do arquivo já gravado.
    conhecidas = {p.lower() for p in coluna(FAMILIAS)}
    sem_familia = sorted({p for _, p, _, _ in linhas if p.lower() not in conhecidas})
    if sem_familia:
        print("provedor sem família em internal/tabelas/provedor_familia.tsv:")
        for p in sem_familia:
            quantos = sum(1 for _, prov, _, _ in linhas if prov == p)
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
    orfaos = sorted({c for c, _, _, _ in linhas if c not in ibge})
    if orfaos:
        print(f"códigos ausentes de municipios.tsv: {', '.join(orfaos)}")
        return 1

    antes = DESTINO.read_text(encoding="utf-8") if DESTINO.exists() else ""
    # A quarta coluna só vai quando marcada: as linhas sem API própria ficam
    # iguais às de antes da coluna existir.
    novo = "\n".join("\t".join(linha if linha[3] else linha[:3]) for linha in linhas) + "\n"
    if conferir:
        if antes != novo:
            a, n = set(antes.split("\n")) - {""}, set(novo.split("\n")) - {""}
            print(f"{DESTINO.relative_to(RAIZ)} desatualizado: {len(n - a)} a mais, {len(a - n)} a menos")
            print("Rode 'make acbr-tabelas' e reveja o diff.")
            return 1
        print(f"{DESTINO.relative_to(RAIZ)} em dia com o fonte pinado")
        return 0
    DESTINO.write_text(novo, encoding="utf-8")

    velhas, novas = set(antes.split("\n")) - {""}, set(novo.split("\n")) - {""}
    proprias = sum(1 for linha in linhas if linha[3])
    print(f"{DESTINO.relative_to(RAIZ)}: {len(linhas)} municípios com provedor,"
          f" {proprias} com API própria")
    if antes:
        print(f"  {len(novas - velhas)} linha(s) a mais, {len(velhas - novas)} a menos")
    return 0


if __name__ == "__main__":
    sys.exit(main())
