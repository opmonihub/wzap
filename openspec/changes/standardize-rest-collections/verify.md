# Verificação — standardize-rest-collections

Evidências coletadas em 2026-10-07, após implementação, no worktree `.worktrees/standardize-rest-collections`, branch `codex/standardize-rest-collections`, base `d95ef6c`. Entrega **BREAKING** de backend, Manager, Swagger e documentação em conjunto.

## Check 1 — Escopo e organização: PASS

Chi v5.3.2 é o único router de produção. `httpapi` compõe o servidor padrão e os pacotes `instances`, `messages`, `groups`, `contacts`, `channels`, `chats`, `statuses`, `profile`, `users`, `authsession`, `media` e `chatwoot`. Os handlers reais pertencem aos recursos; core e representation não importam handlers. As interfaces de serviços são definidas por consumidor; a raiz agrega essas interfaces para wiring. `chats/mark-read` pertence a chats.

Revisão de código e build confirmaram dependências acíclicas e ausência de ServeMux, adaptadores de roteamento ou bridges da API privada. Chi é a única dependência adicionada; tidy também classificou corretamente dependências AWS já utilizadas e retirou transitivas não utilizadas, sem upgrade destas. Não houve alteração de storage, migrações, sessões, eventos, workflows, payloads de escrita, package.json ou lockfile do Manager.

## Check 2 — Serialização e coleções: PASS

As dez operações usam coleções diretas em data: instances, users, groups, messages, channels, messages em mensagens de canal, messages em atualizações de canal, statuses, contacts e blocked_jids. Fontes nil produzem arrays obrigatórios vazios; arrays aninhados preservam events, participantes e JIDs. Opcionais ausentes são omitidos, enquanto false, 0, "0" e [] obrigatórios permanecem.

Os testes HTTP/DTO verificam ausência contra null, settings ausente/parcial, timer "0", Chatwoot persistido desativado, webhook obrigatório desativado, erro com data desconhecida, campos de mensagem/aceite, cursor presente/final e exclusão de segredos. `TestResponseContractListDetailEquivalence` compara JSON decodificado de instâncias, usuários, grupos, mensagens e canais no mesmo estado. `TestResponseContractEmptyCollections` inclui contacts por POST com fonte nil. Nenhum teste depende da ordem das propriedades JSON; replay literal continua comparando os bytes armazenados.

REDs iniciais foram observados pelo especialista na sessão: opcionais null, items/wrappers, fallback raiz 401 e Allow ausente. O coordenador também observou cinco testes novos do Manager falharem com acessos a items e passarem após correção. Não existe arquivo de log separado dos REDs iniciais de Go; os resultados finais foram preservados.

## Check 3 — Roteamento, autorização e replay: PASS

`TestFinalHEAD` usa servidor HTTP real para GET/HEAD em health e em instâncias com e sem autenticação, verificando status, headers e ausência de corpo HEAD. Os testes também verificam 404 JSON na raiz, 401 antes do fallback privado e 405/Allow incluindo HEAD.

O resolver recebe explicitamente o nome do parâmetro ancestral e autoriza o alvo antes da idempotência. O detalhe de mensagem é `/instances/{instance}/messages/{id}`: UUID/nome da instância e UUID da mensagem são independentes. Swagger documenta ambos como parâmetros distintos e obrigatórios. URLs concretas permanecem iguais.

Parâmetros escapados são decodificados uma vez por PathParam. A regressão `%40` foi reproduzida antes da correção; o webhook público por nome usa o mesmo alvo canônico. Testes de escopo e replay verificam rejeição antes de aquisição/reprodução, conflito de fingerprint e corpo armazenado sem transformação. Nenhum registro de replay foi convertido ou apagado.

## Check 4 — Manager: PASS

| Gate | Resultado |
| --- | --- |
| `pnpm --dir manager test` | 3 arquivos, 7 testes passaram |
| `pnpm --dir manager lint` | Exit 0; dois avisos existentes em DataTableToolbar.vue e PageState.vue |
| `pnpm --dir manager typecheck` | Exit 0; repetido após completar o catálogo de tipos |
| `pnpm --dir manager build` | Exit 0; geração Nuxt estática concluída |

QA em navegador sobre build estático e mock local isolado confirmou settings e chatwoot_config omitidos, timer "0", mensagem direta com retry_count=0 e opcionais ausentes, detalhe navegável, ausência de Load more sem cursor e console sem erros. O coordenador inspecionou a captura do detalhe de mensagem. Mock e navegador foram encerrados; não houve chamada real ao WhatsApp nem nova réplica do gateway.

## Check 5 — Swagger e documentação: PASS

Swagger descreve 89 operações em 72 paths. `chi.Walk` confere as 88 operações explicitamente registradas; o documento inclui também a entrada estática `/manager/`. `TestSwaggerSerializationPresence` verifica as dez coleções como arrays obrigatórios, next_cursor opcional e obrigatoriedade/ausência dos campos compartilhados.

O gerador fixado foi executado e repetido com bytes idênticos: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs`. O aviso de ausência de arquivos Go na raiz não impediu geração (exit 0). README e AGENTS descrevem o contrato final e a organização Chi; exemplos de mensagens usam id, send_status e retry_count reais.

| Arquivo | SHA256 |
| --- | --- |
| docs/docs.go | `70ee188f2e5d8226eef8206ade02838674486d536ecaee709a7c76f06bb793a4` |
| docs/swagger.json | `e796c175aff4d1b650062a97343bdbea77d58a72b7a84ffee66a45395e3834bc` |
| docs/swagger.yaml | `34b7b08ae73f63d4b00bfaee0828988bb30597d8f632b968f4c4cf3c83a4fa45` |

## Check 6 — Qualidade e integrações: PASS

Gates Go executados com `GOTOOLCHAIN=go1.26.0`; golangci-lint v2.13.2 instalado (binário compilado com Go 1.27). Todos retornaram exit 0:

| Gate | Evidência |
| --- | --- |
| gofmt | Nenhum arquivo listado |
| `go vet ./...` | Sem erros |
| `golangci-lint run ./...` | 0 issues |
| `go test ./... -count=1` | Suite completa; HTTP 11.210s |
| `go build ./...` | Sem erros |
| Suite completa com Postgres/NATS | HTTP 12.519s; postgres 51.465s; events 0.201s; webhook 1.794s |
| `git diff --check` | Sem erros |

Integração real usou `WZAP_TEST_DATABASE_URL` definido, Postgres acessível em 5435, banco wzap_test e schemas isolados via postgrestest. `WZAP_TEST_NATS_URL` apontou para broker temporário em 32768, sem volumes nem stream compartilhado; container removido após a suite. Não se iniciou uma segunda réplica wzap.

Após retirar import/atribuição mortos em authsession/routes.go, testes HTTP completos passaram novamente (3.175s na raiz), lint global permaneceu com 0 issues e build passou; houve gofmt no arquivo e diff check. A limpeza não alterou comportamento e não repetiu a integração já concluída.

Manifest local de comandos, versões, exit codes e logs: `/tmp/wzap_chi_gates_final.json`; saídas em `/tmp/wzap_all_final.log`, `/tmp/wzap_integration_final.log` e logs referenciados pelo manifest. Os resultados e hashes relevantes estão registrados aqui para não depender da permanência de /tmp.

## Check 7 — Revisão, OpenSpec e entrega: PASS

Cinco agentes Luna auxiliaram revisão Manager, documentação, contrato JSON, roteamento/segurança e QA de UI, em ondas. Revisões finais não encontraram discrepância funcional; as lacunas de teste apontadas foram fechadas: contacts nil, equivalência lista/detalhe, required/optional Swagger e HEAD real. O coordenador revisou diff, escopo, parâmetros finais, schemas, resultados dos gates e documentação. Não há achado bloqueador pendente.

Proposta, design, cinco deltas de specs, tarefas e plano foram reconciliados com as decisões aprovadas. `openspec validate standardize-rest-collections --strict` passou. A change permanece ativa; arquivamento é etapa separada.

A implementação está no worktree, sem commit, merge, publicação ou reinício do serviço em execução. Alterações prévias da árvore principal foram preservadas; nela foram sincronizados apenas os artefatos desta change. Rollback restaura backend e Manager/documentação juntos, sem operação de banco.

Limites: não houve teste de pareamento/envio real WhatsApp, chamada real Chatwoot, deploy ou integração S3 desta change. A QA de UI usa mock, e as integrações reais executadas foram Postgres/NATS. Esses testes externos adicionais não fazem parte do escopo aprovado.

## Finalização da branch — 2026-10-07

Antes de apresentar as opções de integração, a suíte completa foi executada novamente no mesmo worktree: `GOTOOLCHAIN=go1.26.0 go test ./... -count=1` retornou exit 0 (HTTP 11.291s), e `pnpm --dir manager test` retornou exit 0 (3 arquivos, 7 testes). Esta repetição usou as integrações opcionais desativadas; a execução real Postgres/NATS registrada no Check 6 não foi repetida.

Git confirmou worktree em branch nomeada `codex/standardize-rest-collections`, separado do diretório Git comum. O ponto de bifurcação é `d95ef6c`, o HEAD atual de main. A integração aguarda a escolha explícita do usuário; não houve commit, merge, push ou remoção do worktree nesta etapa.
