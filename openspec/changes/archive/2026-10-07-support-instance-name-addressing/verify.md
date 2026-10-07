# Verificação

Data: 2026-10-05. Worktree `.worktrees/support-instance-name-addressing`, base `eaa1cdc`. Evidências, relatórios, revisões, screenshots e proteção da main arquivados em `/home/obsidian/dev/wzap/.superpowers/sdd/finished-support-instance-name-addressing-6x4o0s_p`; estado operacional em `/tmp/wzap-name-state.json`. O registro inicial precedeu revisão final/merge; fatos de integração estão abaixo.

## Sete verificações

1. **Escopo — PASS:** UUID/nome em caminhos de instância, stats direcionado, nomes válidos/únicos e formulários coerentes; três implementadores com escopos separados, mudanças locais e migration 00007 do usuário excluídas.
2. **Especificações — PASS:** `openspec validate support-instance-name-addressing --strict`; três deltas e proposta/design/tasks/plan, política nova marcada BREAKING; nomes antigos e UUIDs preservados.
3. **Comportamento — PASS:** RED/GREEN domínio, roteador real, replay autorizado, seis regressões determinísticas de atualização concorrente com nome legado; PostgreSQL nomes/concorrência/compatibilidade realmente executado com WZAP_TEST_DATABASE_URL definida e schemas isolados em banco wzap_test alcançável; sete testes reais do helper Manager, typecheck/build e lint de arquivos alterados.
4. **Swagger — PASS:** 89 operações existentes, 73 ligadas a instâncias com UUID/nome e 409 ambíguo, filtro opcional instance, correlação e autorização global preservadas; geração exata da CI com swag v1.16.6 repetida com hashes idênticos.
5. **Qualidade completa — PASS:** gofmt vazio, go vet ./..., golangci-lint v2.13.2, go test ./... -count=1, go build ./..., diff-check, freshness com exit0; branch-gates.json registra comandos/tempos/hashes. Suíte geral sem envs DB/NATS, integrações PostgreSQL apenas no escopo selecionado acima. Host Go1.27; Dockerfile usa Go1.26.
6. **Revisão — PASS:** três revisões de tarefa aprovadas, P2 de restauração concorrente de nome inválido corrigido e re-revisão aprovada; revisão final independente APPROVED, sem findings acionáveis, com fontes/73 operações/hashes conferidos.
7. **Integração/visual — PASS:** merge local fast-forward 493f644; 48 caminhos locais preservados, 40 byte-exatos, seis merges automáticos byte-exatos, conflito PostgreSQL resolvido mantendo device_jid/GetByDeviceJID e recursos da tarefa, mais método GetByName exigido em fake local não rastreado com reversão byte-exata; revisão independente APPROVED. Main: gofmt/vet/lint/suíte Go/build/typecheck/helper7/testes locais Manager PASS; PostgreSQL nomes/compatibilidade realmente executado novamente com banco/schema isolados (5.924s), geração Swagger sem diff. Imagem limpa Go1.26 wzap:instance-name-20261005, somente app recriado, containers DB/NATS mantidos; health/ready200. Seis requests UUID/nome retornam respostas idênticas200. Browser: reload, Authorize uma vez, executar GET instância/status e stats por nome FELIPE retorna200, chave automática e total1 no filtro.

## Limites

Não há migração ou reescrita de dados. Unicidade cobre escritores transacionais do repositório; SQL direto e binários antigos podem gerar duplicados, que a busca por nome rejeita. Nomes legados inválidos permanecem operáveis por UUID e editáveis sem renomeação. Integração NATS não executada contra stream compartilhado, nem chamadas reais WhatsApp/Chatwoot. Avisos conhecidos de swag raiz sem Go e Nuxt vendor/ssr:false não impedem geração. Browser/componentes não montados nos testes unitários do Manager; integração será registrada separadamente.

## Registro posterior — preservação e execução

Restauração das alterações do usuário exigiu apenas dois hunks no arquivo PostgreSQL: INSERT combina transação/nome com campos/parâmetros device_jid originais; GetByName e GetByDeviceJID convivem. O adapter GetByName do fake local mantém panic e todas as linhas anteriores. Ambos receberam revisão independente; nenhuma mudança local foi incluída nos commits da tarefa. Suite geral continua distinguida das integrações selecionadas. SQL direto e binários antigos continuam fora do protocolo de nomes.

Imagem usa a fonte revisada da branch, sem incluir alterações locais não commitadas ou migration00007. Nenhuma instância existente foi criada, renomeada ou removida para a verificação visual. Nenhum push/PR solicitado.
