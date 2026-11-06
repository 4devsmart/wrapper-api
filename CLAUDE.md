
<!-- fluxo-git-release -->
## Fluxo git

Trabalho em `feat/*` e `fix/*`, integração em `develop`, `main` só guarda
releases. Tudo pelo `make` — `make help` lista.

```bash
make feature name=modulo-assunto   # feat/modulo-assunto a partir de develop (make fix: fix/…)
make sync                          # rebase da branch sobre develop atualizado
make finish                        # rebase, gate UMA vez (make ci), fast-forward em develop, push
make release-dry                   # versão atual, próxima versão e changelog — não muda nada
make release                       # DA PESSOA: bump, CHANGELOG, tag, release/vX.Y.Z, PR para main
make ship-dry                      # o que o ship faria — não muda nada
make ship                          # DA PESSOA: depois da revisão, main anda e o deploy roda
```

**Regras para o agente:**
- **Nunca** rode `make release` nem `make ship` de verdade. Confira com
  `make release-dry` / `make ship-dry` e entregue o comando à pessoa.
- **Nunca** edite `VERSION` nem `CHANGELOG.md` à mão: só o commit
  `chore(release): vX.Y.Z` que `make release` escreve mexe neles (o hook
  `commit-msg` recusa o resto). A versão sai dos commits, não de quem commita.
- Commit em Conventional Commits: `<tipo>(<escopo>): <assunto>`. `feat` sobe o
  minor, `tipo!:` ou rodapé `BREAKING CHANGE:` sobe o major, o resto sobe o patch.
- Não use `--no-verify` para passar por um hook: o hook recusando é a resposta.
- Trabalho novo começa com `make feature`/`make fix`, não com `git checkout -b`.
- A configuração do fluxo (gate, tamanho do assunto, deploy) está em `.workflow.conf`.

### Neste projeto

- O que exige ação de quem atualiza vai no commit, em `Antes-de-atualizar: ...`.
- `make ship` espera a imagem do `publicar.yml`, e o `release.yml` promove as tags.
- Lib nova: `make acbr-libs-publicar` antes do commit que sobe `ACBR_REV`.
- `scripts/release` e `scripts/ship` têm adaptações marcadas com `wrapper-api:`, que
  o `setup.sh` sobrescreve: confira o diff ao atualizar o fluxo.
- Coautoria: commits e PRs sem `Co-Authored-By` nem "Generated with Claude Code".
