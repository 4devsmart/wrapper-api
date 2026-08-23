# Política de Segurança

## Como reportar uma vulnerabilidade

**Não abra uma issue pública** para vulnerabilidades de segurança.

Reporte de forma privada por um destes canais:

1. **GitHub** (preferido) — aba **Security → Report a vulnerability**
   ([Private Vulnerability Reporting]). Cria um canal privado com os mantenedores.
2. **E-mail** — `security@devobjetivo.com.br`.

Inclua, se possível:

- descrição do problema e do impacto;
- passos para reproduzir (PoC), versão/commit afetado;
- configuração relevante (sem segredos reais — **nunca** envie certificados,
  `.env` ou XMLs com dados reais).

## O que esperar

- **Confirmação** do recebimento em até **3 dias úteis**.
- Avaliação e, quando aplicável, uma correção coordenada antes da divulgação
  pública. Pedimos que aguarde a correção antes de divulgar (*coordinated
  disclosure*).
- Crédito ao relator na nota da correção, se desejado.

## Versões suportadas

O projeto está em desenvolvimento ativo; correções de segurança são aplicadas na
versão mais recente (`main` / último release). Não há suporte retroativo a versões
antigas.

## Escopo

Relevante: autenticação/sessão da API e do painel, isolamento multi-tenant (RLS),
tratamento de certificados/segredos, injeção (SQL/INI/XML), e a superfície HTTP
pública. Fora de escopo: vulnerabilidades em dependências de terceiros já
rastreadas (use o fluxo do upstream) e configurações inseguras do próprio operador.

> Há uma análise de segurança interna do projeto em
> [`docs/security-review.md`](docs/security-review.md) (revisão de controles — não
> é um canal de report).

[Private Vulnerability Reporting]: https://docs.github.com/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability

## O modelo de ameaça deste serviço

Este wrapper **não persiste nada**, e isso muda onde o risco mora. O que ele
manipula de mais sensível é o **certificado A1 de terceiros**, que chega no corpo
da requisição que transmite.

Consequências que valem um olhar extra em qualquer contribuição:

- **Log é a única forma realista de um segredo escapar do processo.** Por isso
  `fiscal.Certificado`, `boleto.ContaWS` e `nfse.Credenciais` redigem a si
  mesmos em `String()`/`LogValue()`, e o middleware de log nunca registra corpo
  nem cabeçalho de autorização. Um `%v` distraído desfaz isso.
- **O log da ACBrLib em nível 3+ grava XML e certificado em disco.** O boot
  **recusa** essa combinação com `MODO=producao`. Não afrouxe esse gate.
- **O PFX vai da API ao worker por socket unix** e não sai do host. Se um dia o
  transporte virar TCP, ele precisa ficar preso à rede interna.
- **Um token dá acesso a tudo.** Não há escopo. Quem expõe a API na internet
  precisa de TLS na borda e de restringir quem alcança a porta.
