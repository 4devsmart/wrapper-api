# Alvos do wrapper-api. `make help` lista todos.
#
# Regra que economiza tempo: a lib nativa NÃO se compila no host. O build com
# cgo real acontece no Docker multi-stage (etapa 7 do plano); aqui há um alvo
# build-cgo só para VALIDAR o binding contra .so já compiladas.

SHELL   := /bin/bash
# ACBR_REV é a FONTE ÚNICA da revisão do ACBr: o script de download monta a tag
# do release a partir dela, e o snapshot de chaves do lockstep é regenerado em
# lockstep com ela. Ao bumpar, republique as libs e regenere os snapshots.
ACBR_REV ?= 47859
OUT     := out
PKGS    := ./...
ACBRLIBS ?= ./acbr-libs
APP_IMAGE ?= wrapper-api:dev
LDFLAGS := -X github.com/4devsmart/wrapper-api/internal/platform/versao.Commit=$(shell git rev-parse HEAD 2>/dev/null) \
           -X github.com/4devsmart/wrapper-api/internal/platform/versao.Build=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.DEFAULT_GOAL := help

## build: compila api e fiscal-worker sem cgo (binding stub)
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(OUT)/fiscal-worker ./cmd/fiscal-worker

## build-cgo: compila o worker com a ACBrLib real (exige as .so em ACBRLIBS)
build-cgo:
	@test -f "$(ACBRLIBS)/libacbrcte64.so" || { \
	  echo "faltam as .so em $(ACBRLIBS): rode 'make acbr-libs-baixar'"; exit 1; }
	CGO_ENABLED=1 \
	CGO_LDFLAGS="-L$(abspath $(ACBRLIBS)) -Wl,-rpath,$(abspath $(ACBRLIBS))" \
	go build -trimpath -tags acbrlib -ldflags "$(LDFLAGS)" -o $(OUT)/fiscal-worker-cgo ./cmd/fiscal-worker

## run: sobe a api local (sem worker: /readyz responde 503, é o esperado)
run:
	CGO_ENABLED=0 go run ./cmd/api

## test: testes de unidade
test:
	CGO_ENABLED=0 go test $(PKGS)

## test-cgo: type-check e link do binding cgo contra as .so (não executa a lib)
test-cgo:
	@test -f "$(ACBRLIBS)/libacbrcte64.so" || { \
	  echo "faltam as .so em $(ACBRLIBS): rode 'make acbr-libs-baixar'"; exit 1; }
	CGO_ENABLED=1 \
	CGO_LDFLAGS="-L$(abspath $(ACBRLIBS)) -Wl,-rpath,$(abspath $(ACBRLIBS))" \
	go vet -tags acbrlib $(PKGS)

## openapi: regenera os schemas da spec a partir dos modelos Go
openapi:
	CGO_ENABLED=0 go run ./cmd/gerar-openapi

## openapi-valida: confere que a spec é YAML válido e está descrita
openapi-valida:
	@python3 scripts/validar-openapi.py

## openapi-check: falha se a spec estiver desatualizada em relação aos modelos
openapi-check: openapi-valida
	@cp api/openapi.yaml /tmp/openapi.antes.yaml
	@CGO_ENABLED=0 go run ./cmd/gerar-openapi >/dev/null
	@if ! diff -q /tmp/openapi.antes.yaml api/openapi.yaml >/dev/null; then \
	  echo "api/openapi.yaml está desatualizada: rode 'make openapi' e commite."; \
	  diff -u /tmp/openapi.antes.yaml api/openapi.yaml | head -40; \
	  cp /tmp/openapi.antes.yaml api/openapi.yaml; exit 1; \
	fi
	@echo "openapi.yaml em dia com os modelos"

## vet: go vet (build padrão)
vet:
	CGO_ENABLED=0 go vet $(PKGS)

## fmt: gofmt em tudo
fmt:
	gofmt -w .

## fmt-check: falha se algo estiver fora do gofmt (usado no CI)
fmt-check:
	@saida=$$(gofmt -l .); \
	if [ -n "$$saida" ]; then echo "fora do gofmt:"; echo "$$saida"; exit 1; fi

## tidy: go mod tidy
tidy:
	go mod tidy

## acbr-libs-baixar: baixa as libs nativas dos anexos de release (cache local)
acbr-libs-baixar:
	@ACBR_REV=$(ACBR_REV) ACBR_LIBS_DIR=$(ACBRLIBS) ./scripts/baixar-acbr-libs.sh

## acbr-libs-publicar: publica o cache local como release (uma vez por revisão)
acbr-libs-publicar:
	@ACBR_REV=$(ACBR_REV) ACBR_LIBS_DIR=$(ACBRLIBS) ./scripts/publicar-acbr-libs.sh

## acbr-libs-conferir: valida o cache local antes do build
acbr-libs-conferir:
	@falta=0; for f in libacbrnfse64.so libacbrcte64.so libacbrmdfe64.so \
	                   libacbrnfe64.so libacbrboleto64.so schemas.tar.gz; do \
	  if [ ! -s "$(ACBRLIBS)/$$f" ]; then echo "faltando: $(ACBRLIBS)/$$f"; falta=1; \
	  elif [ "$$(stat -c%s "$(ACBRLIBS)/$$f")" -lt 1000000 ]; then \
	    echo "incompleto: $(ACBRLIBS)/$$f"; falta=1; fi; \
	done; \
	if [ "$$falta" = 1 ]; then echo; echo "Rode 'make acbr-libs-baixar'."; exit 1; fi
	@echo "acbr-libs OK"

## docker-build: constrói a imagem (exige acbr-libs/)
docker-build: acbr-libs-conferir
	docker build \
	  --build-arg APP_COMMIT=$(shell git rev-parse HEAD 2>/dev/null) \
	  --build-arg APP_BUILD=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
	  -t $(APP_IMAGE) .

## up / down: sobe e derruba o stack (api + worker)
up: docker-build
	docker compose up -d
down:
	docker compose down

## limpar: remove artefatos de build
limpar:
	rm -rf $(OUT)

## help: esta lista
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /' | sort

.PHONY: build build-cgo run test test-cgo openapi openapi-check openapi-valida vet fmt fmt-check tidy limpar help \
	acbr-libs-baixar acbr-libs-publicar acbr-libs-conferir docker-build up down
