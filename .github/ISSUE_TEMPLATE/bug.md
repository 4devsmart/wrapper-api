---
name: Bug
about: Algo não funciona como documentado
labels: bug
---

**O que aconteceu**

**O que você esperava**

**Como reproduzir**
Payload mínimo (⚠️ **anonimizado** — nunca cole certificado, `.env`, CNPJ ou XML
reais), rota chamada e resposta recebida.

**Ambiente**
- versão da imagem / commit (`GET /v1/ping`):
- documento (CT-e / MDF-e / NFS-e / boleto):
- para NFS-e, o município (código IBGE) e o que `GET /v1/nfse/municipios/{codigo}` responde:

**Logs**
Do `api` e do `worker`. Confira que não há certificado neles.
