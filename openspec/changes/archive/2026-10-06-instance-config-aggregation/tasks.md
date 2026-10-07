## 1. Contrato dos DTOs (TDD)

- [x] 1.1 Fixar o contrato com testes primeiro — chaves exatas de `instance`/`integration`/`settings`, `null` por bloco, `webhook` fora da raiz e ocultação de `external_ref`/`owner_user_id`/JIDs/hashes/token — em `internal/httpapi/dto_contract_test.go` e `internal/httpapi/response_contract_test.go`, verificado por `go test ./internal/httpapi -run 'Contract' -count=1` vermelho antes e verde depois (detalhes em plan.md §1.1).
- [x] 1.2 Implementar `integrationResponse`/`settingsResponse` e mover `webhook` para `integration.webhook` em `internal/httpapi/dto.go`, verificado pelos testes de contrato e por `gofmt -l internal/httpapi` vazio (detalhes em plan.md §1.2).

## 2. Agregação nos handlers

- [x] 2.1 Montar `integration` e `settings` em `GET /instances/{id}`, `GET /instances`, `POST /instances` e `PATCH /instances/{id}`, com blocos vivos apenas quando `connection.status == "connected"`, `null` por bloco em falha e concorrência limitada (~8) na listagem, verificado por testes de handler cobrindo instância desconectada na listagem, fonte indisponível e ordem preservada dos itens (detalhes em plan.md §2.1).

## 3. Persistência de default_disappearing

- [x] 3.1 Criar `internal/storage/migrations/00009_default_disappearing.sql` com coluna nullable, gravar o eco no `PUT /instances/{id}/chats/default-disappearing` e devolver `null` quando nunca configurado, verificado por testes de repositório/migração com `WZAP_TEST_DATABASE_URL` apontando para banco `_test` alcançável (skipped sem a variável) (detalhes em plan.md §3.1).

## 4. Swagger

- [x] 4.1 Atualizar as anotações dos handlers e regenerar o Swagger com os novos blocos, verificado pelos testes de contrato Swagger e por `swag init --parseInternal -g internal/httpapi/swagger.go -o docs` sem diff pendente (detalhes em plan.md §4.1).

## 5. Manager

- [x] 5.1 Atualizar tipos e consumo em `manager/` (`instance.webhook` → `instance.integration.webhook`, blocos `null` tolerantes no detalhe e na listagem), verificado por `pnpm --dir manager typecheck` e `pnpm --dir manager build` (detalhes em plan.md §5.1).

## 6. Documentação

- [x] 6.1 Atualizar a matriz REST do `README.md` e o `response-matrix.md` da change para `GET /instances` e `GET /instances/{id}`, verificado pela conferência linha a linha contra o JSON do design.md §1 (detalhes em plan.md §6.1).

## 7. Verificação e fechamento

- [x] 7.1 Rodar os gates completos (`gofmt -l .`, `go vet ./...`, `golangci-lint run`, `go test ./... -count=1`, `go build ./...`, `pnpm --dir manager build`) e verificar ao vivo `GET /instances` e `GET /instances/{id}` via curl num container reconstruído, registrando a saída (detalhes em plan.md §7.1).
- [x] 7.2 Produzir `verify.md` e `retrospective.md` antes do PR e arquivar a change por último com `openspec-archive-change`, verificado pela presença dos dois arquivos, pela sincronização dos deltas em `openspec/specs/` e pelo arquivamento como último passo (detalhes em plan.md §7.2). Specs sincronizados em `openspec/specs/`; artefatos arquivados em `openspec/changes/archive/2026-10-06-instance-config-aggregation/`.
