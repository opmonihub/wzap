## Why

O serviço usa `log/slog` espalhado em ~47 arquivos sem fábrica central testável, com `Warn` para skips esperados de dados externos, access log sem filtro (health/swagger viram spam) e telefone/JID em nível `Warn`. Isso gera falsos-positivos em alertas e dificulta operar o serviço. Canonicalizar em `zerolog` com semântica de nível estrita elimina o ruído mantendo o contrato de observabilidade.

## What Changes

- Torna `zerolog` (`github.com/rs/zerolog`, hoje `indirect` v1.35.1) o logger canônico em produção e testes, via fábrica única `internal/logger.New(level, format)`.
- `WZAP_LOG_LEVEL` mantém `debug,info,warn,error` (default `info`); `WZAP_LOG_FORMAT` passa a aceitar `json|console` (default `json`), com `text` como alias deprecated de `console`.
- Aplica semântica de nível: `Info` só lifecycle; `Debug` para skips esperados com `reason` tipado; `Warn` só acionável; `Error` só perda de dado/panic.
- Access log passa a pular `GET /healthz`, `GET /readyz` e `/swagger/*` (ou rebaixá-los para `Debug`).
- Throttle/dedupe nos loops de retry (`relay`, `outbox`, `mirror`, `cleaner`); alerta continua via `/readyz`, não por linha de log.
- Minimiza PII: `Warn/Error` carregam só `instance_id`, `conversation_id`, `message_id`, `event_id`; `phone/jid/vcard` vão para `Debug` ou somem (QR/token/apikey/cookie continuam nunca logados).
- Remove `*Context` dos logs (ctx segue só para cancelamento/DB); `waLogger` do `whatsmeow` e middlewares `Logging/Recover` passam a usar `zerolog`.

## Capabilities

### New Capabilities

(nenhuma — sem capacidade nova observável; a fábrica é detalhe interno e vai em `design.md`)

### Modified Capabilities

- `wzap-operations`: a requirement "Observabilidade mínima" muda — formato `json|console` (+ alias `text`), semântica de nível estrita, access log sem probes, PII minimizada, sem falsos-positivos por skip esperado ou retry em loop.

## Impact

- Código: `cmd/wzap`, `internal/app`, `internal/events`, `internal/message`, `internal/webhook`, `internal/media`, `internal/instance`, `internal/session/whatsmeow`, `internal/httpapi`, `internal/chatwoot/*`, mais ~15 arquivos de teste com helpers `slog` in-memory.
- Dependências: `zerolog` vira `require` direto; `log/slog` sai do caminho de produção.
- Sistemas: coletores de log (formato/console), dashboards e alertas baseados em `Warn` (volume cai), `/readyz` como sinal primário de dependência.
- Sem mudança em REST, eventos versionados, schema ou contratos com consumidores.

## Out-of-Scope

- Mudar envelope de eventos, rotas REST, schema Postgres ou contratos com consumidores.
- Sampling, rotação ou shipper de logs; trocar o `whatsmeow` ou o broker.
- Redigir `plan.md`, `verify.md` ou `retrospective.md` neste change (são convenções agent-enforced, fora do schema CLI).
