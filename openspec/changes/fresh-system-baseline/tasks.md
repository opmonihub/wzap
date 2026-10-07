## 1. Preparação da aplicação futura

- [ ] 1.1 Criar o worktree `.worktrees/fresh-system-baseline` e a branch `codex/fresh-system-baseline`, verificando isolamento e preservação das alterações locais existentes com `git worktree list` e `git status`.
- [ ] 1.2 Produzir `plan.md` com passos TDD por task ID e inventário de caminhos históricos, verificando cobertura dos 11 deltas e ausência de exclusões de fluxos atuais por busca textual apenas.

## 2. Schema inicial e identidade

- [ ] 2.1 Substituir a cadeia histórica por uma migração inicial das 15 tabelas atuais e retirar relatórios/gates/backfills, verificando catálogo, instalação vazia e repetição segura com `go test ./internal/storage/postgres -run TestMigrate -count=1` e Postgres `_test` configurado.
- [ ] 2.2 Exigir ownership existente e nomes globalmente únicos com exclusão restrita do dono, verificando constraints, claims concorrentes e isolamento com os testes reais de repositórios em schemas isolados.
- [ ] 2.3 Remover fixtures/testes de upgrade e tolerância histórica e adaptar fixtures válidas ao modelo novo, verificando `go test ./internal/storage/... -count=1` com Postgres `_test` configurado.
- [ ] 2.4 Remover ambiguidade e exceções de nomes históricos preservando UUID/nome exato, reservas, renomeação, quotas e autorização, verificando `go test ./internal/instance ./internal/httpapi -count=1`.

## 3. Boot, credenciais, configuração e eventos

- [ ] 3.1 Simplificar o seed para criar somente o primeiro admin e retirar adoção/compensações de ownership, verificando os testes de seed, criação sem dono válido e reinício com contas existentes.
- [ ] 3.2 Exigir chave Chatwoot quando habilitado e cifrar tokens não vazios sem passthrough ou backfill, verificando chave ausente/inválida, round-trip, adulteração, configuração desligada e ausência de segredos em respostas com testes de configuração e repositórios reais.
- [ ] 3.3 Retirar alias `text`, referências a envs anteriores e handler dedicado de prefixo antigo, verificando apenas json/console válidos e os testes genéricos de autenticação e routing.
- [ ] 3.4 Remover a limpeza vestigial de publicações e seu contrato sem efeito preservando outbox pendente, retries e event_id, verificando `go test ./internal/events ./internal/webhook -count=1` e a integração NATS dedicada.

## 4. Contrato REST e idempotência

- [ ] 4.1 Remover conversores e respostas especiais de replay histórico preservando status/corpo originais, autorização, TTL, fingerprints e liberação de chaves, verificando testes de idempotência atuais incluindo aliases, concorrência e ausência de segundo efeito.
- [ ] 4.2 Remover legacy_error e assegurar código/mensagem/instante reais em falhas atuais, verificando testes de conexão, envio, outbox, dead letters e DTOs sem fabricação de timestamps.
- [ ] 4.3 Remover o campo reservado de revogação e manter sucesso data.revoked e falhas atuais, verificando os testes de lifecycle e fixtures do contrato HTTP.
- [ ] 4.4 Retirar testes de envelopes históricos preservando contratos e privacidade atuais, verificando `go test ./internal/httpapi ./internal/message ./internal/app ./internal/session/... -count=1`.

## 5. Mídia e Manager

- [ ] 5.1 Remover media-migrate, transferência histórica, atualização de bucket para conversão e fallback de bucket vazio, verificando `go test ./cmd/wzap ./internal/media ./internal/storage/postgres -count=1` e referências sem consumidores remanescentes.
- [ ] 5.2 Manter S3 configurado ou disco sem endpoint com bucket explícito, checksum, download, TTL e cache, verificando operações locais/S3, bucket registrado e falha S3 sem gravação alternativa em disco.
- [ ] 5.3 Atualizar formulários, tipos, traduções e mensagens do Manager para o contrato atual após o contrato Go, verificando nomes inválidos/conflitos, falhas estruturadas e `pnpm --dir manager test`, `lint`, `typecheck` e `build`.
- [ ] 5.4 Atualizar README, instruções vigentes e Swagger após remover anotações de compatibilidade, verificando rotas/schemas atuais e regeneração limpa de `docs/` com o comando Swag definido no projeto.

## 6. Gates e revisão antes do reset

- [ ] 6.1 Executar gofmt limpo, `go vet ./...`, golangci-lint v2.13.2, `go test ./... -count=1` com WZAP_TEST_DATABASE_URL configurada e `go build ./...`, registrando comandos, versões, serviços alcançados e resultados reais.
- [ ] 6.2 Executar integrações em broker NATS e bucket S3 dedicados junto dos gates completos do Manager e Swagger, registrando serviços reais separados de fakes e cenários não exercitados.
- [ ] 6.3 Revisar o diff e inventário para confirmar retirada de todo suporte histórico e preservação de segurança, import operacional e pending:{uuid}, registrando evidências file:line e resolvendo findings antes do reset.

## 7. Reset local sem backup e fechamento

- [ ] 7.1 Após os gates, parar a única réplica local e remover somente wzap_pgdata, wzap_natsdata, wzap_miniodata e wzap_media sem backup, verificando mounts e a ausência desses quatro volumes antes da recriação.
- [ ] 7.2 Reconstruir e iniciar uma única réplica com volumes vazios e recriar wzap_test, verificando `/healthz`, `/readyz`, seed, login, criação de instância e mídia no ambiente novo.
- [ ] 7.3 Validar novo pareamento, envio e webhook reais com número de teste, registrando evidência de QR/sessão, mensagem e entrega sem expor dados pessoais ou segredos e deixando a tarefa pendente se o número não estiver disponível.
- [ ] 7.4 Produzir verify.md com sete checks e retrospective.md após a aplicação e revisão, verificando cobertura de todas as tarefas, falhas resolvidas e distinção entre integrações reais, fakes e skips.
- [ ] 7.5 Sincronizar deltas e atualizar Purpose das specs principais que descrevem remodelagem, arquivando somente após concluir tarefas, verify e retrospective e verificar `openspec validate --all --strict`.
