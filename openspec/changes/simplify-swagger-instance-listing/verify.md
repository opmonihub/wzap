# Verificação

Data: 2026-10-05. Implementação isolada em `.worktrees/simplify-swagger-instance-listing`, base `1a99b8f`. Evidências de execução/revisão/browser arquivadas em `/home/obsidian/dev/wzap/.superpowers/sdd/finished-simplify-swagger-instance-listing-ycxoqk1v`; preservação da main em `/tmp/wzap-simplify-state.json`.

## Sete verificações

1. **Escopo — PASS:** contrato items-only, listagem completa, remoção de inputs Swagger; três implementadores com escopos separados, mudanças locais do usuário excluídas da branch.
2. **Especificações — PASS:** `openspec validate simplify-swagger-instance-listing --strict`; proposal/design/deltas/tasks/plan coerentes, contrato **BREAKING** documentado.
3. **Comportamento — PASS:** RED antes das mudanças demonstrou truncamento HTTP e SQL, validação de cursor obsoleto, headers duplicados e repetição do overview; GREEN dos cinco pacotes focados, quatro regressões reais de composables, typecheck e build Nuxt; Postgres List/Backfill realmente executado com variável de teste definida, banco `_test` alcançável e schemas isolados, coleção com 123 linhas e ordenação determinística.
4. **Swagger — PASS:** testes do documento servido, todas as 89 operações, esquema de autorização preservado, inputs manuais ausentes, items-only e paginação das outras coleções mantida; duas gerações com bytes idênticos via comando exato da CI.
5. **Qualidade completa — PASS:** `gofmt -l .` vazio, `go vet ./...`, `golangci-lint run` v2.13.2, `go test ./... -count=1` e `go build ./...` com exit 0; DB/NATS envs removidas apenas na suíte geral. Go local 1.27.0; imagem Docker construída com Go 1.26 do Dockerfile, sem alterar go.mod.
6. **Revisão — PASS:** três revisões de tarefa SPEC PASS/QUALITY APPROVED e revisão final independente APPROVED; nenhum finding acionável.
7. **Integração/visual — PASS:** merge fast-forward `102c59a` na main; oito sobreposições locais integradas com provas de remoção do delta da tarefa reproduzindo os originais, mais adaptação de uma única assinatura List de fake em teste local não rastreado, mantendo seu corpo e todas as demais linhas. As 48 alterações anteriores continuam locais. Gofmt/vet/lint/suíte Go/build, typecheck e testes Manager passaram novamente na main; PostgreSQL List/Backfill executado novamente com variável/banco de teste alcançável (3.495s). Geração Swagger na main sem diff. Somente o container wzap foi recriado com a imagem revisada; IDs dos containers Postgres/NATS iguais. Health/ready 200. Browser: reload, Authorize uma vez, Try it out/Execute200, chave enviada automaticamente, nenhum dos quatro inputs, data.items com 2 itens sem next_cursor e X-Request-Id na resposta; requisição com limit=1 e cursor inválido retorna a mesma coleção completa.


## Limites

Não executar integração NATS que reconcilia o stream do serviço ativo. Suite geral não representa execução de toda a integração Postgres: apenas List/Backfill foram executados com o banco neste escopo. Não testar chamadas reais WhatsApp/Chatwoot. Avisos conhecidos do gerador (raiz sem arquivos Go) e build Nuxt (ssr:false/upstream) não impediram os checks.

SHA-256 dos documentos: docs.go `782610056b79d14d5a71b3e969e2c0f90ce57a30d964762ac44e7be4bcda4f27`; swagger.json `b5127ea969b4f028f7ad4daa686897bea8c83c21ff77392b836358f6d9052ccd`; swagger.yaml `61938c5299e3a2ec7864097fad93775620e048ce616f40c600bf20fbbca8bc41`.

## Registro posterior à integração

O primeiro vet na main detectou `stubInstanceRepo.List` em `internal/session/whatsmeow/device_bind_test.go:38`, teste local do usuário ausente da worktree limpa, com assinatura anterior. A causa foi confirmada na declaração; a única assinatura foi atualizada para o contrato novo, sem alterar o corpo. Revisão independente APPROVED verificou reversão exata ao backup original. Esse arquivo continua não rastreado e não foi commitado. Falha inicial e gates posteriores PASS estão separados nas evidências. O teste focado de vínculo do dispositivo também passou (0.008s).

A imagem usa somente a implementação revisada da branch, incluindo Nuxt e Swagger; mudanças locais ainda não commitadas permanecem no workspace. OpenSpec completo e validado; sincronização/arquivo seguem etapa própria do fluxo, sem push/PR.

Limpeza concluída: worktree e branch da tarefa removidas normalmente após o merge. Relatórios, logs, screenshots e backups de proteção ficaram no arquivo de evidências acima. Nenhum conteúdo exclusivo foi descartado.
