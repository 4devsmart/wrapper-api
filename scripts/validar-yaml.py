#!/usr/bin/env python3
"""Confere que todo YAML rastreado é YAML válido.

YAML quebrado não avisa: o GitHub simplesmente ignora o arquivo. Um workflow
que não parseia nunca roda, e some da aba Actions sem mensagem de erro; um
ISSUE_TEMPLATE/config.yml que não parseia derruba o link de reportar
vulnerabilidade em silêncio, num projeto que recebe certificado alheio.

A causa é sempre a mesma: escalar simples com ": " no meio.

    - name: fumaça: a lib carrega     # o parser lê dois mapeamentos
    - name: "fumaça: a lib carrega"   # certo

Já mordeu duas vezes aqui, as duas em texto em português escrito à mão. Havia
gate para api/openapi.yaml (validar-openapi.py) e para mais nada, então os dois
arquivos ficaram quebrados no repositório sem ninguém notar.
"""
import subprocess
import sys

import yaml


def rastreados():
    saida = subprocess.run(
        ["git", "ls-files", "-z", "*.yml", "*.yaml"],
        capture_output=True, text=True, check=True,
    ).stdout
    return sorted(p for p in saida.split("\0") if p)


def main():
    arquivos = rastreados()
    if not arquivos:
        print("nenhum YAML rastreado", file=sys.stderr)
        return 1

    quebrados = []
    for arq in arquivos:
        with open(arq, encoding="utf-8") as fh:
            try:
                yaml.safe_load(fh)
            except yaml.YAMLError as e:
                marca = getattr(e, "problem_mark", None)
                onde = f"linha {marca.line + 1}, coluna {marca.column + 1}" if marca else "?"
                quebrados.append((arq, onde, getattr(e, "problem", str(e))))

    for arq, onde, motivo in quebrados:
        print(f"{arq}: YAML inválido ({onde}): {motivo}", file=sys.stderr)

    if quebrados:
        print(
            f"\n{len(quebrados)} de {len(arquivos)} arquivos YAML não parseiam. "
            'Quase sempre é escalar sem aspas com ": " no meio.',
            file=sys.stderr,
        )
        return 1

    print(f"{len(arquivos)} arquivos YAML, todos válidos")
    return 0


if __name__ == "__main__":
    sys.exit(main())
