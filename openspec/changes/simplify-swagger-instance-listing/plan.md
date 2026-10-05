# Plano de implementação

Contrato e decisões: proposal.md, design.md e specs/. Requisitos já definidos pelo usuário; executar sem nova rodada de aprovação. Integração local na main segue a escolha prévia do usuário; não publicar PR nem push.

## 1.1 Backend (implementador A)

Escopo: internal/httpapi/instances.go e testes de instâncias/stats/auth; internal/instance/service.go e seus fakes/testes; internal/storage/repository.go, postgres/instances.go e testes relacionados; internal/session/whatsmeow/manager.go e fakes; internal/chatwoot/import/scheduler.go e testes; demais fakes internos estritamente necessários para compilar o novo List(ctx).

1. Escrever regressões HTTP que retornam todos os itens (>100), não têm next_cursor, ignoram limit/cursor inválidos e mantêm escopos, e uma regressão SQL para listagem completa e ordem determinística; executar e registrar falha esperada antes da implementação.
2. Alterar List(ctx) para ([]model.Instance,error); eliminar constantes, parse e query de cursor da listagem de instâncias. Manter ErrInvalidCursor para outras coleções.
3. Adaptar stats, restauração e scheduler para a consulta única, preservando comportamento e cancelamento; ajustar fakes e testes antigos de páginas.
4. Remover os dois @Param de header em instances.go e atualizar descrição/schema da listagem; nenhuma outra anotação é escrita por este implementador.
5. Verificar: go test ./internal/httpapi ./internal/instance ./internal/session/whatsmeow ./internal/chatwoot/import -count=1 (testes Swagger podem aguardar geração pela tarefa 3.1); WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5435/wzap_test?sslmode=disable' go test ./internal/storage/postgres -run 'TestInstanceRepository(List|Backfill)' -count=1.

## 2.1 Manager (implementador B)

Escopo: manager/app/types/api.ts, composables/useInstances.ts, composables/useOverview.ts, pages/instances/index.vue, pages/accounts/index.vue, components/instances/InstancesTable.vue (somente plumbing de load-more) e teste existente de overview, se houver. Não tocar package.json, lockfile, traduções ou os componentes locais modificados pelo usuário.

1. Alterar InstanceListPage para InstanceList (items somente) e listInstances() sem argumentos/query.
2. Substituir loops de páginas por uma consulta nas telas e overview; retirar loadMore/nextCursor, preservar estados de erro, busca, ordenação e paginação visual.
3. Adaptar testes existentes, se presentes; verificar pnpm --dir manager typecheck e pnpm --dir manager build com dependências da worktree.

## 3.1 Swagger (implementador C)

Escopo: arquivos anotados internal/httpapi/*.go EXCETO instances.go e testes de backend; internal/httpapi/swagger_contract_test.go; docs/docs.go, docs/swagger.json, docs/swagger.yaml; README.md.

1. Fortalecer teste de contrato para proibir inputs apikey/X-Request-Id, exigir segurança nas operações protegidas, proibir limit/cursor/next_cursor na listagem e manter headers de resposta; executar o teste e registrar a falha antes da mudança.
2. Remover somente @Param apikey e @Param X-Request-Id dos arquivos em escopo; preservar @Security e @Header. Registrar **BREAKING** e shape da listagem no README.
3. Depois de A concluir instances.go, gerar via go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs; executar go test ./internal/httpapi -run TestSwagger -count=1 e repetir geração comprovando bytes idênticos.

## 4.1 Revisão e qualidade (controlador)

Revisão de cada tarefa e revisão final independente, sem escrita de código pelos revisores. Executar gofmt -l ., go vet ./..., golangci-lint run, go test ./... -count=1 e go build ./...; consolidar evidências em verify.md (sete verificações) e retrospective.md (evidências e seis seções). Commit convencional somente após revisão e checks.

## 4.2 Integração e teste visual (controlador)

Salvar alterações locais existentes, integrar o commit da worktree na main com fast-forward e reaplicar deltas locais em arquivos compartilhados; comprovar preservação. Gerar a imagem do serviço local a partir da worktree revisada e atualizar apenas wzap, usando o Compose existente; o build não inclui os deltas locais ainda não commitados do usuário. Usar agent-browser em sessão própria para abrir Swagger, preencher Authorize uma vez, executar GET /instances e conferir ausência de inputs apikey/X-Request-Id/limit/cursor, envio automático do header e resposta 200 com data.items sem next_cursor. Reexecutar os checks relevantes à integração caso os deltas locais alterem arquivos compartilhados. Remover worktree/branch após o merge e guardar evidências fora de arquivos versionados. Não executar testes NATS que reconfigurem o stream do serviço ativo.
