# Tasks — whatsmeow-parity-routes (contrato de escopo)

> Detalhe de implementação vive em `plan.md` (referenciado por ID). Verificar
> cada item com o comando ou comportamento observável indicado.

## 1. Fase 1 — leituras + edição (T1)

- [ ] 1.1 Expor `POST /instances/{id}/messages/edit` idempotente — verificar com `go test ./internal/httpapi/ -run TestEditMessage -count=1`.
- [ ] 1.2 Expor `GET /instances/{id}/groups` paginado por cursor — verificar com `go test ./internal/httpapi/ -run TestListJoinedGroups -count=1`.
- [ ] 1.3 Expor `GET /instances/{id}/groups/invite-preview` sem entrar — verificar com resposta `200` + prévia e grupo inalterado.
- [ ] 1.4 Expor `POST /instances/{id}/contacts/check` em lote (≤50) — verificar com `go test ./internal/httpapi/ -run TestCheckContacts -count=1`.
- [ ] 1.5 Expor foto, dispositivos e perfil business de contato — verificar com `GET .../photo|devices|business` retornando `200`/`404`/`422` conforme spec.
- [ ] 1.6 Expor `GET /instances/{id}/blocklist` e `GET /instances/{id}/status/privacy` — verificar com `200` em instância conectada e `409` desconectada.
- [ ] 1.7 Expor mensagens/updates de canal paginados — verificar com `GET .../newsletters/{channel}/messages` + `next_cursor`.

## 2. Fase 2 — moderação e ajustes (T2)

- [ ] 2.1 Expor pedidos de entrada (`GET` + `POST .../requests` approve/decline) — verificar com `go test ./internal/httpapi/ -run TestGroupRequests -count=1`.
- [ ] 2.2 Expor `PATCH /instances/{id}/groups/{group_id}/settings` com allowlist — verificar com `422` fora da allowlist e `403` sem permissão no grupo.
- [ ] 2.3 Expor `POST /instances/{id}/blocklist` (block/unblock) idempotente — verificar com replay devolvendo o original.
- [ ] 2.4 Expor temporizador por conversa + padrão (`PUT .../disappearing`) — verificar com `422` fora de `0`/24h/7d/90d.
- [ ] 2.5 Expor `POST /instances/{id}/contacts/{jid}/subscribe` pontual — verificar com ausência de heartbeat (sinais isolados).
- [ ] 2.6 Expor `GET /instances/{id}/contact-link` com `revoke` — verificar com link antigo invalidado após `revoke=true`.

## 3. Fase 3 — escrita de canal (T3)

- [ ] 3.1 Expor `POST /instances/{id}/newsletters` (criar canal) idempotente — verificar com `go test ./internal/httpapi/ -run TestCreateNewsletter -count=1`.
- [ ] 3.2 Expor `POST .../{channel}/mute` e `POST .../{channel}/viewed` — verificar com `422` para `server_ids` vazio ou >100.
- [ ] 3.3 Expor `POST .../{channel}/reactions` (vazio remove) — verificar com remoção confirmada para emoji vazio.

## 4. Transversal (T4)

- [ ] 4.1 Aplicar envelope, dual auth, RBAC/ownership e `X-Request-Id` em todas as rotas — verificar com `401` sem credencial e `403` sem ownership.
- [ ] 4.2 Documentar rotas no Swagger com erros por operação — verificar com `swag init --parseInternal` sem diff e teste de frescura verde.
- [ ] 4.3 Rodar gates completos — verificar com `gofmt -l .` vazio, `go vet ./...`, `golangci-lint run` e `go test ./... -count=1`.
