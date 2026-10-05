# Verificação

Data: 2026-10-05. Worktree `.worktrees/support-instance-name-addressing`, base `eaa1cdc`. Evidências em `.superpowers/sdd/plan/evidence`, relatórios e revisões ao lado. Este registro precede revisão final, merge e teste visual; fatos posteriores serão anexados.

## Sete verificações

1. **Escopo — PASS:** UUID/nome em caminhos de instância, stats direcionado, nomes válidos/únicos e formulários coerentes; três implementadores com escopos separados, mudanças locais e migration 00007 do usuário excluídas.
2. **Especificações — PASS:** `openspec validate support-instance-name-addressing --strict`; três deltas e proposta/design/tasks/plan, política nova marcada BREAKING; nomes antigos e UUIDs preservados.
3. **Comportamento — PASS:** RED/GREEN domínio, roteador real, replay autorizado, seis regressões determinísticas de atualização concorrente com nome legado; PostgreSQL nomes/concorrência/compatibilidade realmente executado com WZAP_TEST_DATABASE_URL definida e schemas isolados em banco wzap_test alcançável; sete testes reais do helper Manager, typecheck/build e lint de arquivos alterados.
4. **Swagger — PASS:** 89 operações existentes, 73 ligadas a instâncias com UUID/nome e 409 ambíguo, filtro opcional instance, correlação e autorização global preservadas; geração exata da CI com swag v1.16.6 repetida com hashes idênticos.
5. **Qualidade completa — PASS:** gofmt vazio, go vet ./..., golangci-lint v2.13.2, go test ./... -count=1, go build ./..., diff-check, freshness com exit0; branch-gates.json registra comandos/tempos/hashes. Suíte geral sem envs DB/NATS, integrações PostgreSQL apenas no escopo selecionado acima. Host Go1.27; Dockerfile usa Go1.26.
6. **Revisão — PASS:** três revisões de tarefa aprovadas, P2 de restauração concorrente de nome inválido corrigido e re-revisão aprovada; revisão final independente APPROVED, sem findings acionáveis, com fontes/73 operações/hashes conferidos.
7. **Integração/visual — PENDING:** proteção de 48 caminhos locais intacta; merge, gates da main e Swagger no serviço serão registrados após execução.

## Limites

Não há migração ou reescrita de dados. Unicidade cobre escritores transacionais do repositório; SQL direto e binários antigos podem gerar duplicados, que a busca por nome rejeita. Nomes legados inválidos permanecem operáveis por UUID e editáveis sem renomeação. Integração NATS não executada contra stream compartilhado, nem chamadas reais WhatsApp/Chatwoot. Avisos conhecidos de swag raiz sem Go e Nuxt vendor/ssr:false não impedem geração. Browser/componentes não montados nos testes unitários do Manager; integração será registrada separadamente.
