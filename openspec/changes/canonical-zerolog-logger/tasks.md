## 1. Fábrica canônica

- [x] 1.1 Promover `zerolog` a dependência direta e criar `internal/logger` com parse de nível/formato e verificar com `go test ./internal/logger/ -count=1`
- [x] 1.2 Cobrir `json|console` e alias `text→console` com deprecation e verificar com `go test ./internal/logger/ -run TestNew -count=1`

## 2. Boot e pontes

- [x] 2.1 Migrar `cmd/wzap` para a fábrica e verificar boot com `WZAP_LOG_FORMAT=json` e `console` emitindo no formato esperado
- [x] 2.2 Reescrever `waLogger` sobre `zerolog` preservando `module/Sub` e verificar com `go test ./internal/session/whatsmeow/ -count=1`
- [x] 2.3 Migrar middlewares `Logging/Recover` com skip de `healthz/readyz/swagger` e verificar com `go test ./internal/httpapi/ -run TestLogging -count=1`

## 3. Domínio e transporte

- [x] 3.1 Migrar `instance` e fronteira `httpapi/connection` para `zerolog` com branches como constantes e verificar com `go test ./internal/instance/ ./internal/httpapi/ -count=1`
- [x] 3.2 Migrar `app` removendo `*Context` dos logs e verificar com `go test ./internal/app/ -count=1`
- [x] 3.3 Migrar `events`, `message` e `media` com throttle nos loops e verificar com `go test ./internal/events/ ./internal/message/ ./internal/media/ -count=1`
- [x] 3.4 Migrar `webhook` com throttle de delivery/dead-letter e verificar com `go test ./internal/webhook/ -count=1`
- [x] 3.5 Migrar `chatwoot/*` rebaixando skips esperados para `Debug` com `reason` e minimizar PII e verificar com `go test ./internal/chatwoot/... -count=1`

## 4. Testes e docs

- [x] 4.1 Migrar helpers in-memory de `slog` para `zerolog.Nop()` e helper de buffer/JSON preservando guards anti-segredo e verificar com `go test ./internal/instance/ ./internal/httpapi/ ./internal/session/whatsmeow/ -count=1`
- [x] 4.2 Atualizar `README` (`WZAP_LOG_FORMAT json|console`) e verificar com `grep -n "WZAP_LOG_FORMAT" README.md`

## 5. Gates finais

- [x] 5.1 Rodar gates completos e verificar com `go vet ./... && golangci-lint run && go test ./... -count=1 && go build ./... && test -z "$(gofmt -l .)"`
