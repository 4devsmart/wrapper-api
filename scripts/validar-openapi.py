#!/usr/bin/env python3
"""Valida api/openapi.yaml: YAML bem formado, refs resolvidas e tudo descrito.

O piso de descrição existe porque documentar os 1.044 campos foi trabalho de
várias passadas. Sem ele, o primeiro campo novo sem descrição passa
despercebido e a erosão recomeça.
"""
import sys, re, yaml

ARQ = "api/openapi.yaml"

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
    sem_doc = [n for n, e in schemas.items() if not e.get("description")]
    problemas += [f"schema sem description: {n}" for n in sem_doc]

    campos = faltando = 0
    for nome, esquema in schemas.items():
        for campo, valor in (esquema.get("properties") or {}).items():
            if not isinstance(valor, dict) or "$ref" in valor or "allOf" in valor:
                continue  # herda a descrição do tipo apontado
            campos += 1
            if not valor.get("description"):
                faltando += 1
                if faltando <= 15:
                    problemas.append(f"campo sem description: {nome}.{campo}")

    for t in doc.get("tags", []):
        if not t.get("description"):
            problemas.append(f"tag sem description: {t['name']}")

    if problemas:
        print(f"{ARQ}: {len(problemas)} problema(s):", file=sys.stderr)
        for p in problemas[:25]:
            print(f"  · {p}", file=sys.stderr)
        if faltando > 15:
            print(f"  · … e mais {faltando - 15} campos sem description", file=sys.stderr)
        print("\nAcrescente ao comentário do modelo ou a internal/fiscal/glossario.tsv,"
              "\ndepois rode 'make openapi'.", file=sys.stderr)
        return 1

    print(f"openapi.yaml: {len(doc['paths'])} rotas, {len(schemas)} schemas, "
          f"{campos} campos, tudo descrito")
    return 0

if __name__ == "__main__":
    sys.exit(main())
