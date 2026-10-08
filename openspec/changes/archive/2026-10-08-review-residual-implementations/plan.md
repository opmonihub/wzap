# Revisão de problemas residuais — Implementation Plan

> **For agentic workers:** use superpowers:subagent-driven-development para execução por área e revisão cruzada; TDD e systematic-debugging para cada comportamento.

**Goal:** corrigir achados demonstrados da revisão global sem alterar shapes públicos ou perder mudanças locais.
**Architecture:** transportes validam input/replay/autorização; domínio serializa PATCH parcial; envio confirma estado/evento em transação e import reconhece snapshots; clientes preservam credenciais; manager mantém fluxos com campos opcionais. Um writer por arquivo, trabalho na branch codex/review-residual-implementations.
**Tech Stack:** Go 1.26, Chi v5, Postgres/pgx, NATS JetStream, Nuxt 4/Vue/Vitest.
**Spec:** proposal.md, design.md, specs/ e tasks.md desta change.

## Global Constraints

- Sem novas dependências, migrações, workflows, segredo real nos testes ou segunda réplica wzap.
- Go: `PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0`; `gofmt` nos arquivos alterados.
- Integração DB somente banco terminado `_test`, helper postgrestest com schema isolado; NATS exclusivamente broker temporário.
- Não editar AGENTS.md/specs principais copiados como contexto nem alterações locais do usuário; não commit/merge/deploy.
- Arquivos de code/test não podem ser escritos por dois agentes; o coordenador escreve artefatos e integra o patch final.

## Review Focus

- Identidade apagada ou role alterada depois de JWT emitido: nenhuma autorização obsoleta.
- Mesmo Idempotency-Key com alvo, método ou corpo efetivo diferente: 422, zero efeitos adicionais.
- PATCH concorrente com campos omitidos: nenhuma reversão de identidade/configuração.
- Dados sensíveis/credenciais em erro ou redirect: não publicados nem encaminhados.
- Alias e campos omitidos no manager: ações disponíveis e nenhum request com payload inventado.

### Task 1: API e autenticação (tasks 1.1–1.6)

**Ownership:** `internal/httpapi/**` e testes HTTP compartilhados; nenhum outro implementer escreve nessa árvore.
**Files:** core/{middleware,idempotency,instances}.go, composition wiring que chama Authenticate, authsession/login_ratelimit.go, messages/messages.go, statuses/status.go e testes correspondentes; pode adicionar regressões em arquivo dedicado no mesmo pacote.
**Interfaces:** consumir UserRepository existente; preservar assinatura pública REST e context Scope/TargetInstance; internal Authenticate pode receber repositório de usuários.

- [x] RED 1.1: login real -> remoção da conta -> GET/POST admin dá 401; token admin com role atual user dá 403; falha de lookup falha fechado; API key funciona independente. Reusar fakeUserRepository/auth test, sem JWT sem conta real na fixture válida.
- [x] GREEN 1.1: revalidar cookie antes de autorizar e atualizar role do scope; executar regressões.
- [x] RED 1.2: router PUT timer e PATCH group settings executam setter uma vez no replay, 422 divergente/409 em progresso; canal/grupo distinto com mesma chave/body é 422; UUID/nome/rename preservam replay e autorização.
- [x] GREEN 1.2: métodos registrados e fingerprint que conserva parâmetros além da referência da instância; manter hashes de rotas não afetadas quando viável.
- [x] RED 1.3: mudar o teste que exige passthrough corrupto para exigir 500, zero calls e zero release; incluir status zero/inválido, JSON vazio/truncado/sem envelope e erro malformado; bytes íntegros continuam iguais.
- [x] GREEN 1.3: validação mínima da integridade atual, sem DTO-conversão nem recarregar recurso.
- [x] RED 1.4: JSON seguido de segundo valor/junk -> 400; JSON curto + espaços acima de 1 MiB -> 413; desconhecidos e whitespace final pequeno aceitos; zero efeito para os inválidos.
- [x] GREEN 1.4: um Decode adicional exige EOF e respeita MaxBytesReader; FingerprintRequest também recusa JSON acima do limite antes de Acquire com 413, sem fallback à rota; confirmar zero efeito/aquisição e manter limites multipart próprios.
- [x] RED 1.5: multipart com query concorrente não altera campo do form; inverter campos escalares repetidos muda o alvo efetivo e não pode replayar; incluir arquivo único/duplicatas relevantes.
- [x] GREEN 1.5: campos vêm exclusivamente de multipart e fingerprint inclui sua semântica efetiva; preservar casos válidos existentes.
- [x] RED 1.6: MaxLoginBuckets+1 IPs não aumenta mapa além do teto nem renova budget de IP conhecido; expiração libera capacidade usando relógio existente.
- [x] GREEN 1.6: recusar overflow após limpeza de expirados, sem expulsar budgets ativos.
- [x] Verificar: `go test ./internal/httpapi/... ./internal/auth/... -count=1`; registrar RED/GREEN por task em relatório próprio.

### Task 2: Núcleo, segurança e import (tasks 2.1–2.5, 2.7–2.8, 2.10–2.11)

**Ownership:** internal/instance, internal/session, internal/webhook (exceto worker.go/worker_test.go reservados a Task 6), internal/chatwoot/** e teste de sealing em internal/storage/postgres/chatwoot_test.go; cmd/wzap/main.go somente se necessário habilitar history-sync/lifecycle, com aviso ao coordenador; não editar httpapi/manager ou outros arquivos storage/message/events.
**Files:** instance/service.go e regressões; session/whatsmeow/manager.go e testes de restore/device bind; webhook/{deliver,ssrf}.go e testes; chatwoot/client/client.go e client_test.go; chatwoot/config/token.go e token_test.go; storage/postgres/chatwoot_test.go.
**Interfaces:** Service.Update mantém UpdateInput de ponteiros; locker próprio por instância; SealToken/OpenToken recebem/retornam plaintext na fronteira de repositório.

- [x] Ler probes originais `/tmp/wzap-core-review-probes/overlay.json` para reutilizar evidência sem refazer descoberta.
- [x] RED 2.1: duas calls reais Service.Update com barreiras demonstram webhook-only/partial identity reverting rename e partial webhook reverting enabled/events.
- [x] GREEN 2.1: adquirir locker antes de Get, manter até escrita; não UpdateIdentity quando campos de identidade ausentes; testar cancelamento/erros e não reentrância.
- [x] RED 2.2: bound JID sintético inválido/ausente/mismatch/upstream error não aparece em log nem reason/projeção; errors.Is de sentinels/contexto permanece útil.
- [x] GREEN 2.2: retirar JID na origem e impedir propagar detalhes sensíveis do upstream em reason, manter IDs opacos/categoria segura.
- [x] RED 2.3: dois httptest origins com redirect 307 e downgrade HTTPS->HTTP; destino secundário não é chamado e 3xx é erro.
- [x] GREEN 2.3: no redirect nos dois clientes autenticados e fechamento/tratamento correto do response.
- [x] RED 2.4: plaintext `enc:v1:...` passa round trip e nunca fica intacto no SQL; Get->Put sempre sela; empty/tamper/wrong-key continuam cobertos.
- [x] GREEN 2.4: selar toda entrada não vazia, sem heurística de prefixo.
- [x] RED 2.5: data_url de outro host com `[loopback, IP público confiável]` não alcança servidor loopback; testar redirects e todos os endereços usados no dial, além de host privado Chatwoot legítimo.
- [x] GREEN 2.5: exceção apenas para identidade confiável completa, sem sobreposição parcial de DNS.
- [x] RED 2.7: chunk inserido entre snapshot e conclusão continua pendente; import com erro conserva lote original e chunks novos.
- [x] GREEN 2.7: acrescentar reconhecimento do lote ao HistoryFeed/Accumulator e fakes, sem reset integral que apague dados recebidos depois.
- [x] RED 2.8: import URI ausente não retém chunks/contacts/conversations nem dispara previews/completion; habilitado continua acumulando.
- [x] GREEN 2.8: habilitação explícita da coleta no manager/session e fixtures claras para habilitado/desabilitado.
- [x] RED 2.10: contains retorna telefone alheio mais longo e exato BR; somente variantes exatas são selecionadas/fundidas; grupos/fallback exigem identifier exato.
- [x] GREEN 2.10: filtrar resultados antes de longest/merge e não criar correlação com contato alheio.
- [x] RED 2.11: instance criada sem sessão + init cria/inicia sessão e QR; init:number usa lifecycle/pareamento; status confirma nome/estado sem JID.
- [x] GREEN 2.11: reutilizar lifecycle do serviço, com fakes de contrato atualizados e nenhum novo mecanismo paralelo.
- [x] Verificar focused Go nos pacotes e `go test -race ./internal/instance ./internal/session/whatsmeow ./internal/webhook ./internal/chatwoot/client ./internal/chatwoot/config -count=1`; DB focused quando configurado pelo coordenador.

### Task 3: Manager (tasks 3.1–3.5)

**Ownership:** manager/app e manager/tests, testes/config necessários dentro manager; nenhuma alteração Go ou novas deps.
**Files:** components/instances/{ProfileCard,MessageComposer,ChannelsCard}.vue, pages/instances/[id].vue, components/instances/detail/InstanceSettingsSection.vue, composables de produção relevantes, i18n/locales/en.json e testes que exercitem os fluxos.
**Interfaces:** PATCH profile com campos dirty, mantendo name+status_text verdadeiramente alterados com 501 atômico e aviso; recado-only nunca inclui name. GETs/reloads usam UUID carregado após alias; router.replace mantém query section.

- [x] RED 3.1: executar fluxo real ProfileCard com name igual e recado alterado/limpo, esperar request apenas status_text e sucesso; ambos alterados preservam 501 e aviso sem descarte/sucesso parcial; foto 501 permanece.
- [x] GREEN 3.1: patch de campos alterados e validação só das entradas presentes, recado `""` explícito; nome/recado carregados opcionais com defaults adequados.
- [x] RED 3.2: entry OldName -> rename NewName -> ação/refresh chama UUID e URL válida com section conservada; mudança para outra instância não reutiliza UUID anterior.
- [x] GREEN 3.2: canonicalização apenas necessária e loader usa identidade carregada da instância correta.
- [x] RED 3.3: latitude/longitude vazias, whitespace ou só uma ausente não chama sendLocation; `"0"/"0"` é aceito; bounds continuam validados.
- [x] GREEN 3.3: checar presença antes de Number, manter false/zero legítimos.
- [x] RED 3.4: imagem/vídeo sem legenda conserva linha+Delete e identificação; texto/legenda continuam visíveis e empty só para coleção vazia.
- [x] GREEN 3.4: condicionar conteúdo textual, não a linha completa; copy em en.json se necessária.
- [x] RED 3.5: admin com cache ausente/storage indisponível/freshKey pode revogar; confirmação obrigatória, sucesso limpa cache/one-time key, falha preserva estado e user comum não vê ações.
- [x] GREEN 3.5: gating pelo escopo autorizado e estado de operação, não pelo localStorage.
- [x] Verificar `pnpm --dir manager test`, `node --test manager/tests/instanceName.test.mjs`, lint, typecheck e build; usar harness de componente com dependências existentes, sem testes que só copiem expressões da implementação.

### Task 4: Cobertura restante e revisão (task 4.1)

**Ownership:** read-only; coordenador mantém review.md/tasks/specs/ledger.
- [x] Revisor do núcleo confirma/descarta receipts sem instance_id, snapshot/reset do import, acumulador sem URI e atomicidade send-state/event; cobre lacunas declaradas do domínio/adapters/repositories/Chatwoot.
- [x] Achado confirmado recebe escopo, requisito e RED antes da implementação; hipótese descartada recebe motivo/evidência.
- [x] Revisores cruzados verificam patch final de outra área, com reprodução e testes; corrigir Critical/Important antes de gates.

### Task 5: Gates e entrega local (task 4.2)

- [x] `gofmt` clean, `go test ./... -count=1`, `go vet ./...`, golangci-lint v2.13.2, `go build ./...`, manager test/lint/typecheck/build.
- [x] Suite completa com WZAP_TEST_DATABASE_URL de wzap_test em 5435 e schemas postgrestest; WZAP_TEST_NATS_URL aponta broker temporário separado; sem tocar stream compartilhado; registrar skips externos explicitamente.
- [x] Se annotations mudarem regenerar Swagger pinado, verificar freshness em todos os casos; OpenSpec strict e git diff --check.
- [x] Escrever verify.md (7 checks) e retrospective.md com evidência; sincronizar apenas arquivos da change na árvore principal por patch reversível e verificar hashes preexistentes; remover broker temporário; manter change ativa e worktree revisável sem commit/merge/deploy.
- [x] Rastrear apenas manager/.output/public/.gitkeep, sem assets Nuxt; como nuxt generate remove o arquivo, adicionar passo local de build que o recria usando Node/fs existente, sem deps; verificar build conserva placeholder e Go build/test sem index.html conserva console 503, sem criar teste redundante de arquivo vazio.

### Task 6: Isolamento de recibos e durabilidade terminal (tasks 2.6, 2.9)

**Ownership:** internal/message/**, internal/storage/repository.go, internal/storage/postgres/{messages,events}*.go e testes correspondentes, internal/events/**; consumer fakes em outros pacotes somente coordenados com writer daquele pacote; não editar session/chatwoot/instance/httpapi simultaneamente.
**Files:** message/{receipts,outbox}.go e testes; storage/postgres/messages.go, outbox de eventos e teste transacional; interfaces consumer/storage necessárias; webhook fan-out e cmd wiring somente depois de Task 2 liberar esses arquivos.
**Interfaces:** UpdateReceipt recebe instanceID explícito antes de WhatsApp ID; confirmação terminal persiste state+outbox atomicamente no banco existente, sem novas tabelas/migrações e sem converter contrato REST/event v1.

- [x] RED 2.6: reutilizar probe /tmp/wzap-core-review-wave2/overlay.json com DB _test/postgrestest; duas instâncias e mesmo wa_id alteram somente alvo do receipt.InstanceID; incluir mensagem ausente/sentinels.
- [x] GREEN 2.6: atualizar interface/chamada/predicado SQL e fakes de contrato, mantendo eventos no subject da instância correta.
- [x] RED 2.9: sent/failed não podem commit sem evento durável; simular falha de insert/commit real em schema isolado e comprovar rollback completo; retry de persistência mantém event_id e conta de envio WhatsApp em um.
- [x] GREEN 2.9: transação única para update de mensagem+insert de evento; fan-out somente após commit e sem double-write/double-delivery; retry guarda resultado do envio e evento já construído, sem chamar o sender de novo.
- [x] Verificar `go test ./internal/message ./internal/events ./internal/storage/postgres -count=1` com DB isolado para testes SQL; registrar duração e RED/GREEN em task-6-report.md; revisão cruzada após patch.
