# Verificação — instance-config-aggregation

Data: 2026-10-07. Implementação no working tree de `main` (não commitada). Gates automatizados: [Go gates + tasks.md](0788ef83-d7e2-4c9c-b51f-317303fdb8ed) — relatório `.superpowers/sdd/task-7a-gates-report.md`. Verificação ao vivo: rebuild `docker compose build wzap` + `docker compose up -d wzap` nesta sessão.

## Sete verificações

1. **Escopo — PASS:** blocos `integration` e `settings` na leitura de instância; **BREAKING** move de `webhook` para `integration.webhook`; eco `default_disappearing` via migração `00009` (`instance_chat_settings`); Manager e README no mesmo corte; sem escrita agregada no PATCH além dos campos atuais.

2. **Especificações — PASS (estático):** `proposal.md`, `design.md`, deltas em `openspec/changes/instance-config-aggregation/specs/` e `plan.md`/`tasks.md` coerentes com o JSON alvo do design §1.

3. **Comportamento — PASS (testes):** `go test ./internal/httpapi -run 'Contract|Swagger' -count=1` verde; testes de agregação (desconectada, bloco falho, ordem na listagem, create/update) incluídos na suíte `internal/httpapi`. `go test ./... -count=1` verde. **SKIP:** `TestInstanceRepositoryDefaultDisappearing` com Postgres real — `WZAP_TEST_DATABASE_URL` ausente; probe `postgres://wzap:secret@127.0.0.1:5432/wzap_test` falhou em auth SASL no host.

4. **Swagger — PASS:** `docs/*` regenerados; `go test ./internal/httpapi -run Swagger -count=1` verde (webhook ausente na raiz do schema `instance`; `integration`/`settings` documentados; token Chatwoot omitido no aninhado).

5. **Qualidade completa — PASS:** `gofmt -l .` vazio; `go vet ./...`; `golangci-lint run` (0 issues); `go build ./...`; `pnpm --dir manager typecheck` e `pnpm --dir manager build` — conforme task-7a-gates-report.

6. **Revisão — PARCIAL:** implementação revisada via gates e contrato; revisão formal subagent-driven por tarefa não arquivada neste diretório.

7. **Integração ao vivo — PASS:** após imagem `wzap:dev` rebuild, `GET http://127.0.0.1:8081/instances` com `apikey: dev-wzap-token` → `200`; cada `data.items[].instance` **sem** chave raiz `webhook`; **com** `integration.webhook`, `integration.chatwoot_config` (null quando ausente) e `settings` (`default_disappearing`, `profile`, `privacy`, `status_privacy` null em instância `error`/`disconnected`). Exemplo `GET /instances/d59d942a-005d-4066-9d69-b7d0d54389d5`: status `error`, blocos vivos null, webhook em `integration`. Swagger UI: recarregar após deploy e repetir Try it out em `GET /instances` — deve refletir o mesmo shape (documento servido alinhado aos testes Swagger).

## Limites

Postgres integration tests da migração/eco não executados com banco `_test` alcançável neste host. NATS, WhatsApp e Chatwoot reais não exercitados. Nenhum commit/push/PR nesta verificação. **Pendente fluxo OpenSpec:** sincronizar deltas em `openspec/specs/` e `openspec-archive-change` (task 7.2).
