# Verify — canonical-zerolog-logger

Change: `canonical-zerolog-logger` (branch `sdd/canonical-zerolog-logger`, base `main`,
fork point `d80b939`). Data: 2026-09-16. Todas as 13 tarefas de `tasks.md` (1.1–5.1)
marcadas completas; 17 commits à frente de `main`.

## 1. Gates executados na branch (ordem do CI)

Em 2026-09-16, no worktree `.worktrees/sdd-canonical-zerolog-logger`
(toolchain `go1.27.0`, `go.mod` pinado em `go 1.26.0`, `golangci-lint` v2.13.2 = versão do CI):

- `go vet ./...` — limpo (sem saída, exit 0).
- `golangci-lint run` — `0 issues` (exit 0).
- `go test ./... -count=1` — **verde**: 27 pacotes `ok`, 0 falhas
  (incl. `internal/logger`, `internal/session/whatsmeow`, `internal/httpapi` 5.9s,
  `internal/webhook` 1.5s, `internal/auth` 1.1s). Pacotes sem testes inalterados
  (`internal/model`, `internal/storage`, `migrations`, `internal/version`).
- `go build ./...` — limpo (exit 0), inclusive com `CGO_ENABLED=0` (exit 0).
- `gofmt -l .` — vazio (exit 0).
- `go test ./internal/logger/ -run TestNoSlogInProd -v` — PASS (gate anti-`log/slog`
  em prod); `grep -rn '"log/slog"' internal/ cmd/ | grep -v _test.go` → zero matches.

## 2. Testes de integração (não executados — registrados como skipped)

`WZAP_TEST_DATABASE_URL` e `WZAP_TEST_NATS_URL` **ausentes** no ambiente; conforme
`AGENTS.md`, os testes Postgres são pulados sem a variável e não são reportados
como executados. Os pacotes `internal/storage/postgres` passaram no modo sem-banco.

## 3. Smoke dos dois formatos + rejeições

Via `logger.New` (mesma fábrica usada no boot em `cmd/wzap`):

- `json` → uma linha JSON com `level/info/message` (ex.:
  `{"level":"info","smoke":"json","time":"...","message":"smoke format check"}`).
- `console` → saída human-readable (`INF smoke format check smoke=console`).
- `text` → alias de `console` + exatamente um `WRN WZAP_LOG_FORMAT "text" is
  deprecated, use "console"`.
- Nível inválido (`verbose`) → erro nomeando a variável
  (`invalid WZAP_LOG_LEVEL "verbose": must be one of debug, info, warn, error`).
- Formato inválido (`yaml`) → erro nomeando a variável
  (`invalid WZAP_LOG_FORMAT "yaml": must be one of json, console, text`).

## 4. Comportamento coberto por testes (por tarefa)

- **1.1–1.2** (`internal/logger`): allow-list `debug/info/warn/error`, default
  `json`, alias `text→console` com deprecation única, `NewTestLogger` +
  `AssertNoSecret` (guards anti-segredo: QR/token/apikey/cookie/password/vcard).
- **2.1** (`cmd/wzap`): `newLogger` delega à fábrica; boot falha nomeando a variável
  em nível/formato inválido; `slogBridge` temporário removido na 4.1.
- **2.2** (`session/whatsmeow`): `waLogger` sobre zerolog preservando `module/Sub`;
  JID cru só em `Debug`.
- **2.3** (`httpapi` middleware): `Logging` com skip de `GET /healthz`, `/readyz` e
  `/swagger/`; `Recover` com `request_id` + stack; campos `request_id/method/path/
  status/duration_ms` preservados.
- **3.1** (`instance` + `httpapi/connection`): branches como constantes com strings
  byte-idênticas; QR nunca logado (só `qr_present`/`expires_at`).
- **3.2** (`app`): `*Context` removido dos logs; `jid_present` bool no lugar do JID.
- **3.3** (`events/message/media`): throttle de `Warn` em loops (`stream`/`claim`/
  `publish`, `requeue`/`claim`); `phone` só em `Debug` no `jidresolver`;
  per-event `Error`s sem throttle preservados.
- **3.4** (`webhook`): tentativas 2..7 em `Debug`, só 1 e 8 em `Warn`; dead-letter
  exatamente 1 por job esgotado; `queue full` segue `Warn`, `instance gone` → `Debug`.
- **3.5** (`chatwoot/*`): skips rotineiros rebaixados a `Debug` + `reason`;
  `Warn/Error` só com IDs opacos; PII (phone/jid) fora de `Warn+`.
- **4.1**: helpers in-memory unificados em `logger.NewTestLogger()`/`zerolog.Nop()`;
  zero `log/slog` em prod (grep + `TestNoSlogInProd`).
- **4.2**: `README.md` — `WZAP_LOG_FORMAT` documenta `json|console` + alias legado
  (`grep -n` linha 73).

## 5. Contratos preservados (não tocados pela change)

- Envelopes REST (`{"data": …}` / `{"error": {"code","message"}}` + `X-Request-Id`).
- Contrato versionado de eventos (`event_version: 1`, `event_id` estável como
  `Nats-Msg-Id`); entrega at-least-once inalterada.
- Ordem de shutdown e fiação `serve()` intactas (diff restrito a assinaturas de logger).

## 6. Diff resumido

86 arquivos: fábrica nova (`internal/logger/`: `logger.go`, `logger_test.go`,
`slogban_test.go`), migração em `cmd/wzap`, `internal/{app,events,httpapi,instance,
media,message,session/whatsmeow,webhook,chatwoot/...}`, `README.md` + 4 artefatos
do change (`.openspec.yaml`, `design.md`, `plan.md`, `proposal.md`, `specs/`,
`tasks.md`). Sem **BREAKING**.

## 7. Pós-merge

Reexecutar `go test ./... -count=1` sobre o resultado do merge antes de qualquer
cleanup; com `WZAP_TEST_DATABASE_URL`/`WZAP_TEST_NATS_URL` setados, incluir os
testes de integração.
