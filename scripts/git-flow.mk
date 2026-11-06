# Os alvos do fluxo git — os mesmos em todo projeto. O Makefile do projeto o
# inclui com `include scripts/git-flow.mk` e define o alvo `ci` (o gate).
#
#   make feature name=…   branch feat/ a partir de develop   (make fix: fix/…)
#   make finish           rebase, gate UMA vez, fast-forward em develop, develop publicado
#   make release          bump, CHANGELOG, tag, release/vX.Y.Z, PR para main
#   make ship             depois da revisao: main anda, o deploy roda
#
# O pre-push roda o gate so para uma arvore que nunca passou nele.

.PHONY: feature fix chore refactor finish sync check-version release-dry release release-baseline ship-dry ship hooks

## ── Git ─────────────────────────────────────────────────────────────────

feature: ## Branch feat/ nova a partir de develop — make feature name=romaneio-impressao
	./scripts/git-flow start feat "$(name)"

fix: ## Branch fix/ nova a partir de develop — make fix name=romaneio-peso
	./scripts/git-flow start fix "$(name)"

chore: ## Branch chore/ nova a partir de develop — make chore name=deps-npm
	./scripts/git-flow start chore "$(name)"

refactor: ## Branch refactor/ nova a partir de develop — make refactor name=…
	./scripts/git-flow start refactor "$(name)"

finish: ## Rebase em develop, gate UMA vez, fast-forward em develop e push
	./scripts/git-flow finish

sync: ## Rebase da branch atual sobre develop, atualizado de origin
	./scripts/git-flow sync

check-version: ## VERSION e CHANGELOG so mudam em commit de release
	./scripts/check-version

release-dry: ## Versao atual, ultima release, proxima versao e changelog — nao muda nada
	./scripts/release --dry-run $(bump)

release: ## Corta a release de develop e abre o PR para main — make release [bump=major|minor|patch]. NAO e do agente.
	./scripts/release $(bump)

release-baseline: ## Uma vez, ao adotar o fluxo: marca em develop a versao de onde contar — [version=X.Y.Z]
	./scripts/release --baseline $(version)

ship-dry: ## O que `make ship` faria — nao muda nada
	./scripts/ship --dry-run

ship: ## Depois da revisao do PR: main anda para a release e o deploy roda. NAO e do agente.
	./scripts/ship

hooks: ## Liga os hooks versionados em .githooks/ (uma vez por clone)
	git config core.hooksPath .githooks
	@echo "Hooks ligados: $$(ls .githooks | tr '\n' ' ')"
