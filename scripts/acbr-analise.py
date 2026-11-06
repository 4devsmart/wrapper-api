#!/usr/bin/env python3
"""Relatório do que muda ao atualizar o ACBr: lê o que acbr-analise-coleta.sh trouxe.

Responde às perguntas que decidem um bump, antes de compilar qualquer coisa: o que
mudou por área, se os patches locais ainda fazem sentido, quais municípios trocam
de provedor e qual nota técnica cada ambiente passa a usar.

Aplicar sem conflito NÃO prova que o patch é necessário, e aplicar com conflito não
prova que foi resolvido: o relatório aponta onde olhar, e a conclusão é de quem lê.

Uso:  make acbr-analise [ALVO=49010]
      python3 scripts/acbr-analise.py <diretório-da-coleta>
"""
import pathlib
import re
import sys

BLOCO = re.compile(r"^r(\d+) \| (\S+) \| (\d{4}-\d{2}-\d{2})", re.M)
CODIGO_IBGE = re.compile(r"\d{7}")
SEPARADOR = "-" * 72
# O Jenkins commita o número de build: é ruído em todo log da ACBrLib.
RUIDO = re.compile(r"Jenkins atualizando Build Number", re.I)
ALERTA = re.compile(r"\bNT[ .]*\d|Nota T[ée]cnica|Reforma|RTC|layout|leiaute", re.I)


def ler(caminho):
    return caminho.read_text(encoding="utf-8", errors="replace")


def commits(texto):
    """Devolve (revisão, data, mensagem) sem os commits de ruído."""
    saida = []
    for bloco in texto.split(SEPARADOR):
        cabeca = BLOCO.search(bloco)
        if not cabeca:
            continue
        linhas = [
            linha.strip() for linha in bloco.splitlines()
            if linha.strip()
            and not BLOCO.match(linha)
            and not re.match(r"\s*(Changed paths|Caminhos alterados)", linha)
            and not re.match(r"\s+[AMDR] /", linha)
        ]
        mensagem = " ".join(linhas)
        if not RUIDO.search(mensagem):
            saida.append((int(cabeca.group(1)), cabeca.group(3), mensagem))
    return saida


def ler_ini(caminho):
    secoes, atual = {}, None
    for linha in caminho.read_text(encoding="latin-1").splitlines():
        cabeca = re.fullmatch(r"\[(.+)\]\s*", linha)
        if cabeca:
            atual = secoes.setdefault(cabeca.group(1), {})
        elif atual is not None and "=" in linha:
            chave, valor = linha.split("=", 1)
            atual[chave.strip()] = valor.strip()
    return secoes


def municipios(base, alvo):
    """Só provedor e versão: é o que decide classe e layout."""
    saida = []
    for codigo, novo in alvo.items():
        if not CODIGO_IBGE.fullmatch(codigo):
            continue
        antigo = base.get(codigo, {})
        de = (antigo.get("Provedor", ""), antigo.get("Versao", ""))
        para = (novo.get("Provedor", ""), novo.get("Versao", ""))
        if de != para:
            saida.append((codigo, novo.get("Nome", ""), novo.get("UF", ""), de, para))
    return sorted(saida)


def notas_tecnicas(secoes):
    return {
        nome: (dados.get("ProVersaoNT", ""), dados.get("HomVersaoNT", ""))
        for nome, dados in secoes.items()
        if dados.get("ProVersaoNT", "0") not in ("", "0")
        or dados.get("HomVersaoNT", "0") not in ("", "0")
    }


def rotulo(provedor, versao):
    if not provedor:
        return "sem provedor"
    return f"{provedor} {versao}".strip()


def linhas_do_relatorio(raiz):
    base_rev = ler(raiz / "rev-base.txt").strip()
    alvo_rev = ler(raiz / "rev-alvo.txt").strip()
    yield f"# ACBr: r{base_rev} → r{alvo_rev}\n"

    yield "## Commits por área\n"
    yield "| Área | Commits |\n|---|---|"
    todos = {}
    for log in sorted(raiz.glob("log_*.txt")):
        lista = commits(ler(log))
        area = log.stem.removeprefix("log_").replace("_", "/")
        yield f"| `{area}` | {len(lista)} |"
        for rev, data, mensagem in lista:
            todos[rev] = (data, mensagem)

    yield "\n## Mudanças de leiaute a ler com atenção\n"
    achados = [(r, d, m) for r, (d, m) in sorted(todos.items()) if ALERTA.search(m)]
    for rev, data, mensagem in achados:
        yield f"- r{rev} ({data}): {mensagem[:200]}"
    if not achados:
        yield "Nenhuma."

    yield "\n## Patches locais\n"
    yield "| Patch | Arquivo | Mudou no alvo | Aplica sem fuzz |\n|---|---|---|---|"
    for linha in ler(raiz / "patches.txt").splitlines():
        patch, arquivo, mudou, aplica = linha.split("\t")
        yield f"| {patch.split('-')[0]} | `{arquivo.rsplit('/', 1)[-1]}` | {mudou} | {aplica} |"
    yield (
        "\nArquivo sem mudança: o patch segue necessário, salvo correção em outro "
        "arquivo. Arquivo com mudança: leia o trecho antes de concluir."
    )

    base = ler_ini(raiz / "servicos-base.ini")
    alvo = ler_ini(raiz / "servicos-alvo.ini")

    mudancas = municipios(base, alvo)
    yield f"\n## Municípios que mudam de provedor ou versão: {len(mudancas)}\n"
    if mudancas:
        yield "| Código | Município | UF | De | Para |\n|---|---|---|---|---|"
    for codigo, nome, uf, de, para in mudancas:
        yield f"| {codigo} | {nome} | {uf} | {rotulo(*de)} | {rotulo(*para)} |"

    yield "\n## Nota técnica por provedor (produção, homologação)\n"
    antes, depois = notas_tecnicas(base), notas_tecnicas(alvo)
    yield f"- r{base_rev}: {antes or 'nenhuma'}"
    yield f"- r{alvo_rev}: {depois or 'nenhuma'}"
    divergentes = [nome for nome, (pro, hom) in depois.items() if pro != hom]
    if divergentes:
        yield (
            f"\n**Atenção:** produção e homologação usam NTs diferentes em "
            f"{', '.join(divergentes)}. O que passa em homologação não prova produção."
        )

    yield "\n## Pastas de schemas do Padrão Nacional\n"
    a = set(ler(raiz / "esquemas-base.txt").split())
    b = set(ler(raiz / "esquemas-alvo.txt").split())
    yield f"- Novas: {sorted(b - a) or 'nenhuma'}"
    yield f"- Removidas: {sorted(a - b) or 'nenhuma'}"

    yield "\n## Depois desta análise\n"
    yield (
        "Compile em diretório separado e rode `make acbr-tabelas`, `make acbr-chaves`, "
        "`make enums-conferir`, `make golden-nfse` e `make test-integracao-nfse`. "
        "O roteiro está em `docs/ACBRLIB.md`."
    )


def main():
    raiz = pathlib.Path(sys.argv[1])
    relatorio = "\n".join(linhas_do_relatorio(raiz)) + "\n"
    (raiz / "relatorio.md").write_text(relatorio, encoding="utf-8")
    print(relatorio)


if __name__ == "__main__":
    main()
