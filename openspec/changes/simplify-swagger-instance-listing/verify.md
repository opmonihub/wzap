# Verificação

Data: 2026-10-05. Implementação isolada em `.worktrees/simplify-swagger-instance-listing`, base `1a99b8f`. Evidências de execução e revisão em `.superpowers/sdd/plan/` nessa worktree; preservação da main em `/tmp/wzap-simplify-state.json`.

## Sete verificações

1. **Escopo — PASS:** contrato items-only, listagem completa, remoção de inputs Swagger; três implementadores com escopos separados, mudanças locais do usuário excluídas da branch.
2. **Especificações — PASS:** `openspec validate simplify-swagger-instance-listing --strict`; proposal/design/deltas/tasks/plan coerentes, contrato **BREAKING** documentado.
3. **Comportamento — PASS:** RED antes das mudanças demonstrou truncamento HTTP e SQL, validação de cursor obsoleto, headers duplicados e repetição do overview; GREEN dos cinco pacotes focados, quatro regressões reais de composables, typecheck e build Nuxt; Postgres List/Backfill realmente executado com variável de teste definida, banco `_test` alcançável e schemas isolados, coleção com 123 linhas e ordenação determinística.
4. **Swagger — PASS:** testes do documento servido, todas as 89 operações, esquema de autorização preservado, inputs manuais ausentes, items-only e paginação das outras coleções mantida; duas gerações com bytes idênticos via comando exato da CI.
5. **Qualidade completa — PASS:** `gofmt -l .` vazio, `go vet ./...`, `golangci-lint run` v2.13.2, `go test ./... -count=1` e `go build ./...` com exit 0; DB/NATS envs removidas apenas na suíte geral. Go local 1.27.0; imagem Docker construída com Go 1.26 do Dockerfile, sem alterar go.mod.
6. **Revisão — PASS:** três revisões de tarefa SPEC PASS/QUALITY APPROVED e revisão final independente APPROVED; nenhum finding acionável.
7. **Integração/visual — PENDENTE DE EXECUÇÃO:** imagem `wzap:swagger-simplify-20261005` construída da worktree revisada; deltas de oito arquivos compartilhados preparados, incluindo três conflitos de inserção adjacente resolvidos conservando GetByDeviceJID. Remover o delta da tarefa reproduz byte por byte cada original do usuário; main ainda não alterada neste registro. Merge e Authorize/Execute serão registrados após execução.

## Limites

Não executar integração NATS que reconcilia o stream do serviço ativo. Suite geral não representa execução de toda a integração Postgres: apenas List/Backfill foram executados com o banco neste escopo. Não testar chamadas reais WhatsApp/Chatwoot. Avisos conhecidos do gerador (raiz sem arquivos Go) e build Nuxt (ssr:false/upstream) não impediram os checks.

SHA-256 dos documentos: docs.go `782610056b79d14d5a71b3e969e2c0f90ce57a30d964762ac44e7be4bcda4f27`; swagger.json `b5127ea969b4f028f7ad4daa686897bea8c83c21ff77392b836358f6d9052ccd`; swagger.yaml `61938c5299e3a2ec7864097fad93775620e048ce616f40c600bf20fbbca8bc41`.
