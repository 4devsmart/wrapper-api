#!/usr/bin/env python3
"""Valida api/openapi.yaml: YAML bem formado, refs resolvidas e tudo descrito.

O piso de descrição existe porque documentar os campos foi trabalho de várias
passadas. Sem ele, o primeiro campo novo sem descrição passa despercebido e a
erosão recomeça.

Ele percorre o documento INTEIRO. A versão anterior olhava só
components.schemas[*].properties, e por isso não via os parâmetros de rota, os
schemas escritos à mão dentro de paths, nem as propriedades aninhadas: dizia
"tudo descrito" com 35 campos sem descrição fora do alcance dela.

Também recusa descrição MUTILADA. O gerador recorta detalhe de implementação do
comentário Go, e recorte deixa rastro: parêntese sem fecho, " .", ",.", artigo
repetido. Como o texto publicado é o produto, um rastro desses é defeito, não
cosmética.
"""
import re
import sys

import yaml

ARQ = "api/openapi.yaml"

# Nomes que pertencem ao código e não ao contrato: quem integra não sabe o que é
# ACBr, e um arquivo .pas da biblioteca não ajuda ninguém a preencher um campo.
RE_INTERNO = re.compile(r"(?i)\bACBr|\bNuvem Fiscal\b|\.pas\b|\bACBrLib")

# Abertura no estilo godoc ("PedidoEmissao é o CT-e..."): o identificador Go não
# existe do lado de fora.
RE_ABERTURA_GO = re.compile(
    r"^([A-Z][A-Za-z0-9]*) (?:é|são|representa|espelha|reúne|agrega|descreve|"
    r"identifica|traz|lista|marca|guarda|contém|modela|define|agrupa) "
)

RE_PONTUACAO = re.compile(r"(\s\.|,\.|:\.|;\.|\s,|/\.)(\s|$)")
RE_REPETIDA = re.compile(r"(?i)\b(a|o|as|os|de|da|do|em|na|no|que)\s+\1\b")


def textos(no, caminho, achados):
    """Coleta (caminho, texto) de todo description/summary do documento."""
    if isinstance(no, dict):
        for k, v in no.items():
            if k in ("description", "summary") and isinstance(v, str):
                achados.append((f"{caminho}.{k}", v))
            else:
                textos(v, f"{caminho}/{k}", achados)
    elif isinstance(no, list):
        for i, v in enumerate(no):
            textos(v, f"{caminho}[{i}]", achados)


def campos_sem_descricao(no, caminho, achados):
    """Toda propriedade de todo objeto, em qualquer profundidade, precisa de
    descrição. $ref e allOf herdam a do tipo apontado e ficam de fora."""
    if isinstance(no, dict):
        for nome, valor in (no.get("properties") or {}).items():
            if isinstance(valor, dict) and not ("$ref" in valor or "allOf" in valor):
                if not valor.get("description"):
                    achados.append(f"{caminho}.{nome}")
        for k, v in no.items():
            campos_sem_descricao(v, f"{caminho}/{k}", achados)
    elif isinstance(no, list):
        for i, v in enumerate(no):
            campos_sem_descricao(v, f"{caminho}[{i}]", achados)


def parametros_sem_descricao(paths):
    fora = []
    for rota, ops in paths.items():
        for metodo, op in ops.items():
            if not isinstance(op, dict):
                continue
            for p in op.get("parameters") or []:
                if isinstance(p, dict) and not (p.get("description") or "").strip():
                    fora.append(f"{metodo.upper()} {rota} -> {p.get('name')}")
    return fora


def mutilacoes(caminho, texto):
    """Rastros de recorte no texto publicado."""
    t = texto.strip()
    fora = []
    if t.count("(") != t.count(")"):
        fora.append("parêntese sem par")
    if RE_PONTUACAO.search(t):
        fora.append("pontuação solta")
    if RE_REPETIDA.search(t):
        fora.append("palavra repetida")
    if RE_INTERNO.search(t):
        fora.append("nome interno vazando")
    if RE_ABERTURA_GO.match(t):
        fora.append("abre com identificador Go")
    return [f"{caminho}: {m}\n      {t[:110]}" for m in fora]


def main() -> int:
    bruto = open(ARQ, encoding="utf-8").read()
    try:
        doc = yaml.safe_load(bruto)
    except yaml.YAMLError as e:
        print(f"{ARQ}: YAML inválido: {e}", file=sys.stderr)
        return 1

    problemas = []
    schemas = doc["components"]["schemas"]

    # refs resolvidas
    for tipo, nome in set(re.findall(r'\$ref:\s*"#/components/(\w+)/([^"]+)"', bruto)):
        if nome not in doc["components"].get(tipo, {}):
            problemas.append(f"$ref pendente: {tipo}/{nome}")

    # schema declarado e nunca referenciado é peso morto
    raizes = {"Erro"}
    referenciados = {n for _, n in re.findall(r'\$ref:\s*"#/components/(schemas)/([^"]+)"', bruto)}
    for nome in schemas:
        if nome not in referenciados and nome not in raizes:
            problemas.append(f"schema órfão (declarado e não referenciado): {nome}")

    # tudo descrito
    problemas += [f"schema sem description: {n}" for n, e in schemas.items() if not e.get("description")]

    sem_campo = []
    campos_sem_descricao(doc, "", sem_campo)
    problemas += [f"campo sem description: {c.lstrip('/')}" for c in sem_campo[:15]]

    problemas += [f"parâmetro sem description: {p}" for p in parametros_sem_descricao(doc["paths"])]

    for t in doc.get("tags", []):
        if not t.get("description"):
            problemas.append(f"tag sem description: {t['name']}")

    # descrição mutilada
    todos = []
    textos(doc, "", todos)
    for caminho, texto in todos:
        problemas += mutilacoes(caminho.lstrip("/"), texto)

    if problemas:
        print(f"{ARQ}: {len(problemas)} problema(s):", file=sys.stderr)
        for p in problemas[:25]:
            print(f"  · {p}", file=sys.stderr)
        if len(sem_campo) > 15:
            print(f"  · … e mais {len(sem_campo) - 15} campos sem description", file=sys.stderr)
        print("\nAcrescente ao comentário do modelo ou a internal/fiscal/glossario.tsv,"
              "\ndepois rode 'make openapi'.", file=sys.stderr)
        return 1

    print(f"{ARQ}: {len(doc['paths'])} rotas, {len(schemas)} schemas, "
          f"{len(todos)} textos, tudo descrito e sem rastro de recorte")
    return 0


if __name__ == "__main__":
    sys.exit(main())
