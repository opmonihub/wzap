# instance-config-aggregation — plano de implementação (TDD)

## Global Constraints

- Contrato alvo e decisões fechadas: `design.md` §1–§7; nada fora do escopo de `tasks.md`.
- **BREAKING**: `webhook` só em `integration.webhook`; consumidor único (`manager/`) migra no mesmo corte.
- Semântica: `null` por bloco quando indisponível; blocos vivos (`profile`, `privacy`, `status_privacy`) só quando `connection.status == "connected"`; listagem nunca falha por uma instância; concorrência limitada (~8) por requisição na lista.
- `integration.chatwoot_config` é `null` sem configuração persistida; token Chatwoot write-only; `external_ref`, `owner_user_id`, JIDs e hashes nunca saem.
- `settings.default_disappearing` = eco do último PUT aceito (`0`/`24h`/`168h`/`2160h`), `null` quando nunca configurado; migração `00009_default_disappearing.sql`, coluna nullable, sem backfill.
- Testes Postgres só com `WZAP_TEST_DATABASE_URL` para banco `_test` alcançável (skipped sem a variável); registrar o que foi realmente executado.
- Cada passo escreve somente seus arquivos; commits convencionais (`test(httpapi):`, `feat(api):`, `feat(storage):`, `docs(specs):`).

## Task 1.1 — Testes de contrato primeiro

Write scope: `internal/httpapi/dto_contract_test.go`, `internal/httpapi/response_contract_test.go`.

1. Adicionar fixtures de contrato para `GET /instances/{id}` e `GET /instances` com os cenários: instância desconectada (blocos vivos `null`), conectada (blocos preenchidos), sem config Chatwoot (`chatwoot_config: null`), sem PUT de timer (`default_disappearing: null`) e eco após PUT.
2. Asserir chaves exatas de `instance` (`id`, `name`, `connection`, `integration`, `settings`, `created_at`, `updated_at`), de `integration` (`webhook`, `chatwoot_config`) e de `settings` (`default_disappearing`, `profile`, `privacy`, `status_privacy`), com `webhook` ausente da raiz.
3. Asserir ocultação: `token`, `external_ref`, `owner_user_id`, `device_jid`, `whatsapp_jid`, hashes nunca presentes em nenhum bloco.
4. Rodar `go test ./internal/httpapi -run 'Contract' -count=1` e confirmar vermelho; commit `test(httpapi): fix instance aggregation contract`.

## Task 1.2 — DTOs e mapeadores

Write scope: `internal/httpapi/dto.go`.

1. Criar `integrationResponse{Webhook webhookResponse, ChatwootConfig *chatwootConfigResponse}` e `settingsResponse{DefaultDisappearing *string, Profile *profileResponse, Privacy *privacyResponse, StatusPrivacy *statusPrivacyResponse}` (formas reutilizando os DTOs já públicos das rotas próprias).
2. Alterar `instanceResponse`: remover `Webhook` da raiz, acrescentar `Integration` e `Settings`; ajustar `newInstanceResponse` para montagem parcial (persistidos) e criar o enriquecimento com blocos vivos/Chatwoot fora do mapper puro.
3. Rodar `go test ./internal/httpapi -count=1` e `gofmt -l internal/httpapi`; commit `feat(api): nest webhook under integration and add settings block`.

## Task 2.1 — Agregação nos handlers

Write scope: `internal/httpapi/instances.go` (handlers de leitura/listagem/criação/atualização), `internal/httpapi/chatwoot.go` (reuso do mapper), fakes em `internal/httpapi/*_test.go`; `internal/instance/` apenas se a interface de leitura precisar de método agregado (sem mudar semântica de domínio).

1. Adicionar testes de handler: listagem com instância desconectada responde 200 com blocos vivos `null`; falha de uma fonte vira `null` só no bloco; ordem de `items` preservada com agregação paralela; criação/atualização respondem com `integration`/`settings`.
2. Implementar montagem compartilhada: `integration.webhook` do agregado, `integration.chatwoot_config` via `ChatwootConfigStore` (`null` em `storage.ErrNotFound`), `settings.default_disappearing` do novo campo persistido.
3. Bloco vivo: quando `connection.status == "connected"`, buscar `GetProfile`/`GetPrivacy`/`GetStatusPrivacy` em paralelo (erro → `null` + log warn); nos demais estados, pular chamadas e responder `null`.
4. Lista: montar itens com concorrência limitada (grupo com limite 8, ex. `errgroup.SetLimit(8)`), preservando a ordem; nenhum erro de agregação propaga status HTTP.
5. Rodar `go test ./internal/httpapi ./internal/instance -count=1`; commit `feat(api): aggregate integration and settings on instance reads`.

## Task 3.1 — Eco persistido de default_disappearing

Write scope: `internal/storage/migrations/00009_default_disappearing.sql`, `internal/storage/repository.go`, `internal/storage/postgres/instances.go` (ou arquivo equivalente de instância), `internal/instance/parity_writes.go`, `internal/httpapi/parity_writes.go` (gravação do eco no PUT), testes de repositório/migração.

1. Adicionar teste de repositório: coluna começa `null`; após `SetDefaultDisappearingTimer` persiste o valor textual aceito; downgrade remove a coluna.
2. Migração `00009_default_disappearing.sql`: `ALTER TABLE instances ADD COLUMN default_disappearing text NULL` (sem backfill; down remove a coluna), registrada no embed do goose.
3. Estender o contrato de storage com leitua/escrita do eco e gravar no caminho de sucesso do `PUT /instances/{id}/chats/default-disappearing` (mesmo allowlist `0`/`24h`/`168h`/`2160h`).
4. Rodar `go test ./internal/storage/... ./internal/instance -count=1` e, com `WZAP_TEST_DATABASE_URL` para um banco `_test` alcançável, `go test ./internal/storage/postgres -count=1`; commit `feat(storage): persist default_disappearing echo`.

## Task 4.1 — Swagger

Write scope: anotações em `internal/httpapi/`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.

1. Atualizar anotações dos handlers de instância (`envelope{data=instanceEnvelope}` com os novos blocos) e garantir que os schemas de resposta não exibam `token`/`external_ref`/`owner_user_id`/JIDs.
2. Regenerar com `swag init --parseInternal -g internal/httpapi/swagger.go -o docs` e rodar os testes de contrato/frescor do Swagger (`go test ./internal/httpapi -run 'Swagger' -count=1`).
3. Commit `docs(api): document integration and settings blocks`.

## Task 5.1 — Manager

Write scope: `manager/app/types/api.ts`, consumidores em `manager/app/` (composables/cards/seções de detalhe), `manager/i18n/locales/en.json` se necessário; sem mudanças de dependência.

1. Atualizar o tipo `Instance`: `webhook` sai da raiz, `integration: {webhook, chatwoot_config: ... | null}` e `settings: {default_disappearing, profile, privacy, status_privacy}` com blocos anuláveis.
2. Trocar todo consumo de `instance.webhook` por `instance.integration.webhook` e tratar blocos `null` como indisponíveis no detalhe e na listagem (sem erro de renderização).
3. Rodar `pnpm --dir manager typecheck` e `pnpm --dir manager build`; commit `fix(manager): consume integration and settings blocks`.

## Task 6.1 — Documentação

Write scope: `README.md` (matriz REST), `response-matrix.md` da change.

1. Atualizar as linhas de `GET /instances` e `GET /instances/{id}` (e a nota de criação/atualização) na matriz REST do README com a representação agregada e o marcador **BREAKING**.
2. Conferir linha a linha contra o JSON do `design.md` §1; commit `docs(readme): refresh instance response matrix`.

## Task 7.1 — Gates e verificação ao vivo

1. Rodar na ordem: `gofmt -l .` (vazio), `go vet ./...`, `golangci-lint run`, `go test ./... -count=1`, `go build ./...`, `pnpm --dir manager build`.
2. `docker compose up -d --build wzap` e conferir com curl `GET /instances` e `GET /instances/{id}` (instância desconectada e conectada) + eco após `PUT /instances/{id}/chats/default-disappearing`; registrar a saída como evidência.

## Task 7.2 — Fechamento

1. Produzir `verify.md` (7 checks, distinguindo executados de skipped) e `retrospective.md` (evidência primeiro) antes do PR.
2. Sincronizar os deltas em `openspec/specs/` e arquivar por último com `openspec-archive-change`.
