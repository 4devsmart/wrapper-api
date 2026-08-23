#!/usr/bin/env python3
"""Compara os enums declarados com os schemas XSD oficiais.

Os XSD vêm no pacote de schemas da biblioteca e são a mesma fonte que a SEFAZ
publica no MOC. Ao bumpar a revisão, este script mostra o que mudou: valor novo,
valor que saiu do layout, ou campo que virou enumerado.

Uso:  python3 scripts/conferir-enums.py [diretório-dos-schemas]
      (default: extrai de acbr-libs/schemas.tar.gz para um temporário)
"""
import glob, pathlib, re, subprocess, sys, tempfile

def schemas(destino=None):
    if destino:
        return destino
    pacote = pathlib.Path("acbr-libs/schemas.tar.gz")
    if not pacote.exists():
        # acbr-libs/ é cache local, fora do versionamento: sem ele não há o que
        # comparar. Não é erro, é só não ter baixado ainda.
        print(f"{pacote} não está aqui. Rode 'make acbr-libs-baixar' e tente de novo.")
        sys.exit(0)
    tmp = tempfile.mkdtemp(prefix="schemas-")
    subprocess.run(["tar", "-xzf", str(pacote), "-C", tmp], check=True)
    return tmp

def do_xsd(raiz):
    """(documento, campo) -> conjunto de valores aceitos, por variante do layout.

    O XSD declara os valores de duas formas: inline, dentro do próprio elemento,
    ou por referência a um tipo nomeado (type="TModTransp"). Resolvemos as duas,
    senão campos como o modal aparecem incompletos.
    """
    tipo_nomeado = re.compile(r'<xs:simpleType name="(\w+)">(.*?)</xs:simpleType>', re.S)
    bloco = re.compile(r'<xs:element name="(\w+)"([^>]*)>(.*?)(?=<xs:element name="|\Z)', re.S)
    out = {}
    for sub, doc in (("schemas-cte", "cte"), ("schemas-mdfe", "mdfe")):
        arquivos = {f: pathlib.Path(f).read_text(encoding="utf-8", errors="replace")
                    for f in glob.glob(f"{raiz}/{sub}/*.xsd")}
        tipos = {}
        for txt in arquivos.values():
            for m in tipo_nomeado.finditer(txt):
                vals = [v for v in re.findall(r'<xs:enumeration value="([^"]*)"', m.group(2)) if v]
                if vals:
                    tipos.setdefault(m.group(1), set()).update(vals)
        for txt in arquivos.values():
            for m in bloco.finditer(txt):
                nome, atributos, corpo = m.groups()
                # O corpo vai até o próximo elemento declarado; se este fechou
                # antes disso, corta no fecho para não herdar o que vem depois.
                corpo = corpo.split("</xs:element>", 1)[0]
                vals = {v for v in re.findall(r'<xs:enumeration value="([^"]*)"', corpo) if v}
                ref = re.search(r'type="(?:\w+:)?(\w+)"', atributos)
                if ref:
                    vals |= tipos.get(ref.group(1), set())
                if vals:
                    out.setdefault((doc, nome), set()).update(vals)
    return out


def declarados():
    """(documento, campo) -> valores que publicamos, seguindo a tag enum.

    Devolve também o conjunto de campos que os modelos expõem, para separar o
    que é lacuna nossa do que é campo de layout que a API sequer aceita.
    """
    enums, tags, nossos = {}, {}, set()
    for doc in ("cte", "mdfe"):
        for l in pathlib.Path(f"internal/{doc}/enums.tsv").read_text(encoding="utf-8").split("\n"):
            if l and not l.startswith("#") and l.count("\t") >= 2:
                nome, valor, _ = l.split("\t", 2)
                enums.setdefault((doc, nome), set()).add(valor)
        for p in pathlib.Path(f"internal/{doc}").glob("*.go"):
            if p.name.endswith("_test.go"):
                continue
            fonte = p.read_text(encoding="utf-8")
            nossos |= {(doc, m) for m in re.findall(r'json:"(\w+)[",]', fonte)}
            for m in re.finditer(r'json:"(\w+)[",][^`]*enum:"(\w+)"', fonte):
                tags[(doc, m.group(1))] = m.group(2)
    return {k: enums.get((k[0], e), set()) for k, e in tags.items()}, tags, nossos

def main() -> int:
    raiz = schemas(sys.argv[1] if len(sys.argv) > 1 else None)
    xsd, (meus, tags, nossos) = do_xsd(raiz), declarados()

    divergentes = 0
    for chave, publicados in sorted(meus.items()):
        oficiais = xsd.get(chave)
        if not oficiais:
            continue
        # O XSD repete o mesmo nome em variantes do layout (GTV-e, CT-e OS,
        # eventos), então o conjunto oficial é a UNIÃO. Só acusamos valor que
        # publicamos e não existe em variante nenhuma: esse é erro certo.
        sobrando = publicados - oficiais
        if sobrando:
            divergentes += 1
            print(f"  {chave[0]}.{chave[1]} (enum {tags[chave]}): publicamos "
                  f"{sorted(sobrando)} que o layout não aceita em nenhuma variante")
    # Só interessa o campo que a API aceita e ainda não documenta os valores.
    # Listas geográficas (UF, código de município) ficam de fora: são tabela
    # do IBGE, não enumeração de negócio.
    GEOGRAFICO = ("UF", "cUF", "cMun", "CEP", "cPais", "xPais")
    faltando = sorted(k for k in xsd
                      if k in nossos and k not in meus and len(xsd[k]) > 1
                      and not k[1].startswith(GEOGRAFICO))
    if faltando:
        print(f"\n  {len(faltando)} campo(s) que aceitamos, enumerados no layout e ainda sem "
              "lista de valores publicada:")
        for d, c in faltando:
            print(f"    {d}.{c}: {sorted(xsd[(d, c)])}")

    if divergentes:
        print(f"\n{divergentes} divergência(s). Corrija internal/<doc>/enums.tsv e rode 'make openapi'.")
        return 1
    print("enums em dia com os schemas oficiais")
    return 0

if __name__ == "__main__":
    sys.exit(main())
