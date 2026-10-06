# Plano de execução — remodel-storage-and-json-responses

## Situação e escopo

Registro aprovado de decisões, com execução **condicionada à tarefa 1.1**. Esta entrega escreve documentação; não aplica migrações nem altera backend, Manager ou infraestrutura. `design.md` é a referência das 14 tabelas, relações, enums e JSON exato. `tasks.md` é o contrato de escopo; `response-matrix.md` inventaria as operações atuais e precisa ser completado antes do corte.

Não confundir a conclusão dos artefatos OpenSpec com prontidão para apply. A imagem MinIO solicitada permanece sem origem verificável após o retorno `no such manifest`; não trocar a tag silenciosamente.

## Sequência e método

Para cada alteração de comportamento abaixo: escrever primeiro teste que demonstra o requisito, executar e observar falha pertinente, implementar o mínimo, executar o teste novamente e refatorar mantendo-o verde. Os cenários são objetivos de testes futuros, não resultados já obtidos. Revisar o diff antes dos commits convencionais propostos. Delegação futura terá escopos de escrita separados; contrato HTTP deve estar fechado antes de dividir Go e Manager entre agentes.

### Etapa 1 — decisões e baseline (1.1–1.2)

1. Completar em `design.md` a matriz de tipos/defaults/NULL, enum SQL, política de updated_at, índices e ações referenciais; documentar owners, órfãos, JIDs conflitantes, IDs bigint e rollback sem perda.
2. Completar `response-matrix.md` por inspeção de `internal/httpapi/`, `docs/swagger.json` e consumidores em `manager/app/`; fixar contratos de ações, agregados, entradas, erros e cache legado.
3. Definir atomicidade das correlações Chatwoot pré-WA ID e tratamento de corpos idempotentes não conversíveis sem novo efeito.
4. Comprovar a origem da imagem exata, definir configuração S3 por ambiente e ensaiar sua disponibilidade; fechar critérios de provisão e credenciais sem gravar segredos.
5. Coordenar migração 00007 e criar branch `codex/` em `.worktrees/`; preservar alterações alheias, coletar preflight e backups, preparar fixtures representativas em schema isolado.

Saída verificável: cinco bloqueios resolvidos documentalmente, todas as operações mapeadas e baseline reproduzível. Commit documental proposto: `docs(specs): finalize storage remodel execution contracts`.

### Etapa 2 — banco e domínio (2.1–2.4)

Escopo: `internal/storage/migrations/`, `internal/storage/repository.go`, `internal/storage/postgres/`, `internal/model/`, `internal/instance/`, `internal/session/whatsmeow/` e serviços consumidores das interfaces alteradas.

1. Criar testes PostgreSQL de upgrade com fixtures das tabelas atuais, conflitos e órfãos, além de banco vazio; implementar migrações conforme a estratégia fechada, sem alterar schemas WhatsMeow/Goose.
2. Testar cardinalidades e referências cruzadas; implementar constraints que rejeitem relações entre instâncias diferentes e preservem associações opcionais.
3. Testar reconexão por device_jid e edição de identidade concorrente com mudança de conexão; separar comandos de escrita para impedir snapshots obsoletos.
4. Testar nomes, enums, unicidades, quotas e retenção de 500 dead letters; atualizar consultas e modelos preservando os significados dos contadores.

Checks focados: `go test ./internal/storage/postgres ./internal/instance ./internal/session/whatsmeow -count=1`, com `WZAP_TEST_DATABASE_URL` apontando para banco acessível cujo nome termine em `_test`; cada teste usa `postgrestest` e schema isolado. Sem essa variável, não declarar integração PostgreSQL executada. Commits propostos: `feat(storage): remodel resource schemas` e `refactor(instance): separate identity connection and webhook writes`.

### Etapa 3 — armazenamento de objetos (3.1–3.4)

Escopo: `internal/config/`, `internal/media/`, repositório de mídia em `internal/storage/postgres/`, `internal/message/`, `internal/app/`, `internal/chatwoot/` e arquivos Compose/configuração de produção existentes.

1. Testar cliente S3 com bytes conhecidos, autorização e falhas; implementar ambiente e serviço MinIO na rede existente com a imagem exata comprovada.
2. Testar upload/download e materialização temporária durante envio e mirror; adaptar os consumidores de Path com cleanup após sucesso, erro e cancelamento.
3. Testar expiração, objeto já ausente, falha SQL após exclusão e repetição do cleaner; preservar metadados e registrar object_deleted_at somente após confirmação.
4. Ensaiar transferência com UUID/checksum/tamanho preservados, retomada e rollback; classificar marcadores Chatwoot sem fabricar wa_id ou conteúdo ausente.

Checks focados: `go test ./internal/media ./internal/config ./internal/message ./internal/chatwoot/... ./internal/app -count=1` e integração real MinIO definida em 1.1. Testes com fake não comprovam a disponibilidade da imagem. Commit proposto: `feat(media): store objects in minio preserving metadata`.

### Etapa 4 — eventos, erros e idempotência (4.1–4.4)

Escopo: `internal/events/`, `internal/webhook/`, `internal/message/`, `internal/chatwoot/`, `internal/httpapi/` e repositórios correspondentes.

1. Testar broker indisponível, retomada e PubAck seguido de exclusão SQL com falha; publicar com ID estável e excluir somente após confirmação.
2. Testar erros novos tipados, ausência e texto legado com occurred_at null; manter a serialização de eventos v1 independente dos DTOs HTTP.
3. Testar captura dos UUIDs de Enqueue pelo Chatwoot, múltiplos anexos e falhas parciais conforme contrato fechado; preservar deduplicação do mirror/import.
4. Testar replay de caches legados, autorização e fingerprint antes do resultado; converter corpos sem novo envio e sem reaparecimento de campos removidos.

Checks focados: `go test ./internal/events ./internal/webhook ./internal/message ./internal/chatwoot/... ./internal/httpapi -count=1`, com fixtures v1 e integração NATS em servidor isolado via `WZAP_TEST_NATS_URL`. Commit proposto: `feat(events): retain only pending outbox events` e `fix(api): convert legacy idempotent responses safely`.

### Etapa 5 — API, Manager e Swagger (5.1–5.3)

Escopo: `internal/httpapi/`, `manager/app/`, `manager/i18n/`, `docs/swagger.json`, `docs/swagger.yaml` e gerados pertinentes.

1. Criar fixtures por operação da matriz final para DTOs, listas vazias, erros, segredos, criação/rotação e exceções; implementar mapeadores explícitos.
2. Atualizar `manager/app/types/api.ts` e consumidores para instance aninhada, instances_used e os nomes aprovados; testar quotas e formulários sem limpar dados ocultos.
3. Atualizar anotações dos handlers e executar o gerador adotado pelo projeto; verificar que schemas, status, exemplos e segurança global refletem os handlers.
4. Exercitar visualmente instâncias, envio, erros, webhook, Chatwoot e contas com Swagger e Manager usando ambiente de teste.

Checks: `go test ./internal/httpapi -count=1`; `pnpm --dir manager test`, `pnpm --dir manager typecheck`, `pnpm --dir manager lint` e `pnpm --dir manager build`. Commits propostos: `feat(api): standardize resource response envelopes` e `feat(manager): consume remodeled resource contracts`.

### Etapa 6 — validação e integração (6.1–6.2)

1. Revisão independente do diff e contratos com evidências file:line, incluindo riscos de migração, ownership, mídia, replay e eventos v1.
2. Executar a ordem CI: `go vet ./...`, `golangci-lint run`, `go test ./...`, `go build ./...`; `gofmt -l .` deve estar vazio. Executar novamente apenas checks justificados por correções ou falhas.
3. Executar integração PostgreSQL (`WZAP_TEST_DATABASE_URL`, banco `_test`), NATS (`WZAP_TEST_NATS_URL`) e MinIO comprovado; distinguir testes ignorados, fakes e serviços realmente exercitados.
4. Ensaiar corte/recuperação e comparar IDs, contagens, constraints, owners, quotas, arquivos e respostas idempotentes; não executar corte real como efeito desta entrega documental.
5. Registrar evidências e limitações em `verify.md`, produzir `retrospective.md`, concluir revisão e integração pelo fluxo do projeto; sincronizar specs e arquivar por último, removendo worktree após merge.

## Critérios para liberar o corte

Todas as tarefas de comportamento verificadas; backups e recuperação ensaiados; imagem exata comprovada; nenhuma referência órfã ou divergência sem tratamento; cobertura de todas as rotas; Manager e Swagger compatíveis; eventos v1 preservados; nenhum replay produz efeito duplicado. O resultado deve marcar explicitamente **BREAKING** para consumidores REST.
