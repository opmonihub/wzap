## Context

Ver `proposal.md` (Why). Estado atual: `log/slog` instanciado em `cmd/wzap/main.go:newLogger` (`WZAP_LOG_LEVEL`, `WZAP_LOG_FORMAT json|text`, saída `os.Stdout`) e injetado como `*slog.Logger` em ~20 pacotes (`app`, `events`, `message`, `webhook`, `media`, `instance`, `session/whatsmeow`, `httpapi`, `chatwoot/*`); ponte `waLogger` (`session/whatsmeow/store.go`) adapta `slog` para `waLog.Logger`; testes usam `slog.Handler` in-memory (`service_logging_test.go`, `connection_logging_test.go`, `pairing_logging_test.go`) e buffers `NewText|JSONHandler`. Restrições: Go 1.26, `CGO_ENABLED=0`, fronteiras em `AGENTS.md` (transport vs domínio vs sessão vs eventos vs mídia vs storage), qualidade (`gofmt`, `go vet`, `golangci-lint`, `go test ./...`, `go build ./...`).

## Goals / Non-Goals

**Goals:** um logger canônico com fábrica única testável; semântica de nível que elimina falsos-positivos sem perder diagnóstico; PII minimizada; testes migrados sem perder os guards anti-segredo.

**Non-Goals:** sampling/shipper/rotação; mudar REST, eventos versionados, schema; trocar broker ou `whatsmeow`.

## Decisions

- **Fábrica `internal/logger.New(level, format) (zerolog.Logger, error)` em vez de configurar `zerolog` global.** Rationale: um ponto de parse/validação (`debug,info,warn,error`; `json|console` + alias `text→console`), `Timestamp` sempre ligado, saída `os.Stdout`; pacotes recebem `zerolog.Logger` por valor. Alternativa (global `zerolog.SetGlobalLevel` + `log.Logger` mutável) descartada: piora teste paralelo e esconde dependência.
- **Injeção por valor, ausente = `zerolog.Nop()`.** Rationale: elimina os ~15 fallbacks `slog.Default()` que hoje vazam para stderr/texto fora do padrão. Alternativa (ponteiro + nil-check) descartada: recria o mesmo risco.
- **Dropar `*Context` nos logs.** Rationale: nenhum consumidor lê `zerolog.Ctx` hoje; `request_id` já vai como campo explícito (`middleware.go`, `health.go`); ctx segue para cancelamento/DB. Alternativa (`logger.WithContext/Ctx`) descartada: acoplamento sem uso.
- **Mapeamento mecânico:** `Info("m","k",v)` → `Info().Str(...).Msg("m")`, `Err(err)` para erros, `With(...)` → `With().Str(...).Logger()`. Branch/debug de `instance/connect/qr` vira constante para não quebrar os 3 contratos de teste.
- **`waLogger` reescrito sobre `zerolog`** preservando `module`/`Sub(module)`: mantém logs da lib no mesmo JSON sem bifurcar formato.
- **`Logging` com skip list (`healthz`, `readyz`, `swagger/`) + `Recover` em `zerolog`:** corta o spam de probe mantendo `method,path,status,duration_ms,request_id`.
- **Throttle nos loops** (`relay`, `outbox`, `mirror`, `cleaner`, `scheduler`): 1ª ocorrência + contador/backoff; estado via `/readyz`. Sem throttle, NATS/DB down gera `N Warn/s`.
- **Testes:** `slog.New(...Discard)` → `zerolog.Nop()`; asserts de conteúdo via buffer + parse JSON com helper `internal/logger` (`newTestLogger` + `assertNoSecret`); asserts por `level+campos-chave`, não string exata (exceto constantes de branch).

## Risks / Trade-offs

- [Risk] Rebaixar `Warn→Debug` esconde diagnóstico real -> Mitigation: `reason` tipado obrigatório (`unsupported_type`, `missing_correlation`, `inbox_not_provisioned`) + cenário de spec por classe de skip.
- [Risk] `zerolog` chain API muda forma dos campos (tipos!) e quebra parse de coletor -> Mitigation: manter nomes de campos (`instance_id`, `event_id`, `request_id`, `error`); smoke `json` vs `console` no apply.
- [Risk] 180 call sites em um change só gera diff grande -> Mitigation: migração por pacote em commits TDD separados (fábrica → pontes → pacotes → testes), ordem em `tasks.md`.
- [Risk] Testes acoplados a mensagem exata ficam vermelhos -> Mitigation: constantes de branch + helper; só os 3 contratos mantêm string exata.
- [Risk] `text` como alias mascara erro de config -> Mitigation: logar 1 `Warn` de deprecation no boot quando `text` for usado.

## Migration Plan

1. Promover `rs/zerolog` a `require` direto; criar `internal/logger` + testes.
2. Migrar `cmd/wzap` (fábrica + `waLogger` + middlewares) primeiro, validar `json|console|text`.
3. Migrar pacotes em ordem: `session/whatsmeow`, `instance`, `httpapi`, `app`, `events/message`, `webhook/media`, `chatwoot/*`.
4. Migrar helpers de teste e religar guards anti-segredo; rodar gates completos.
5. Rollback: reverter commits por pacote (sem mudança de contrato externo, só logs).

## Open Questions

Nenhuma que mude specs, abordagem ou tasks. Detalhe deferido ao apply: valor exato do intervalo de throttle por loop (padrão inicial: 1ª imediata + 1/min por chave).
