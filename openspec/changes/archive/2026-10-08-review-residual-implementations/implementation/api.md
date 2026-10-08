# Task 1 — API e autenticação

Implementação das tasks 1.1–1.6 concluída no worktree `/home/obsidian/dev/wzap/.worktrees/review-residual-implementations`, branch `codex/review-residual-implementations`. Sete bugs confirmados corrigidos, sem novas dependências, migrações, alterações de Swagger ou commits. Árvore principal não foi escrita. Nenhum agente adicional foi criado.

## Causas e correções

| Task / bug | Causa demonstrada | Correção / evidência |
| --- | --- | --- |
| 1.1 / A | Authenticate aceitava role e existência apenas do JWT. Login real seguido de DELETE da conta ainda permitia GET /users (200) e POST /users (201); demotion também permitia criar usuário. Erro de lookup ainda chegava à leitura da instância. | `core/middleware.go:131` consulta Users antes da rota privada: removida → 401, falha/nil repo → 500, role atual no scope. API keys continuam sem depender do lookup de Users. Wiring em server.go. Regressões em `session_account_test.go:17`, `:59`, `:74`. |
| 1.2 / B | Idempotency saía imediatamente para qualquer método diferente de POST, embora timer PUT e settings PATCH já estivessem envolvidos pelo middleware. Replay, divergência e in-progress executavam quatro setters. | `core/idempotency.go:68` cobre POST/PUT/PATCH apenas nas rotas já registradas como idempotentes. Replay executa setter uma vez; alteração recebe 422 e in-progress 409, sem efeito. `idempotency_routes_test.go:15`. |
| 1.2 / C | A identidade usava somente o pattern, perdendo channel/group_id/chat. Dois canais ou grupos da mesma instância com corpo/chave iguais recebiam replay 200. | `core/idempotency.go:353` conserva o formato original method/pattern e acrescenta nomes/valores de recursos concretos com length prefix. Somente o parâmetro na posição /instances/{...} é omitido: abrange id e instance sem perder outro recurso. `idempotency_routes_test.go:60`, `:88`; aliases UUID/nome, rename, old alias 404 e chave estrangeira 403 cobertos. |
| 1.3 / D | Replay escrevia qualquer body/status armazenado, convertendo status zero em 200; retornava vazio, JSON truncado, plaintext, objeto sem envelope e erro malformado. | `core/idempotency.go:174` valida status/envelope antes de escrever, falha 500 sem chamar handler/release ou marcar replay. Sucesso exige data sem error; falha exige error com code/message não vazios sem data. Bytes válidos continuam literais, incluindo whitespace. `core/idempotency_test.go:874`. |
| 1.4 / E | DecodeJSONBody fazia apenas um Decode: JSON válido seguido de segundo JSON/junk/padding >1 MiB executava Enqueue. FingerprintRequest ainda degradava JSON grande à rota antes de Acquire. | `core/instances.go:48` exige EOF no segundo Decode. Fingerprint rejeita não-multipart acima do limite com MaxBytesError, mapeado a 413 antes de Acquire, inclusive se a chave já existe. 400 para conteúdo extra; campos desconhecidos/whitespace/limite exato de 1 MiB aceitos. `request_integrity_test.go:13`, `:55`, `core/idempotency_test.go:857`. |
| 1.5 / F | FormValue dava precedência à query, embora a fingerprint considerasse apenas multipart. Ordenar os valores completos apagava a ordem entre fields/files repetidos, que altera o primeiro consumido. | `messages/messages.go:487` e `statuses/status.go:219` usam PostFormValue. `core/idempotency.go:404` ordena de forma estável por tipo/nome: únicos preservam canonicalização antiga; repetições conservam sua ordem. Query divergente não altera destinatário/caption/filename/PTT/type; inverter fields/files recebe 422 sem segundo upload/send/publish. `multipart_integrity_test.go:31`, `:69`, `:103`, `:129`, `:200`. |
| 1.6 / G | Limiter apenas removia expirados no teto e admitia novos IPs mesmo com 1024 budgets ativos: teste criou 1032 e executou oito handlers indevidos. | `authsession/login_ratelimit.go:46` recusa IP novo sem capacidade após a limpeza, conserva budgets existentes e libera capacidade na expiração. Relógio privado inicializado com time.Now permite avanço determinístico nas regressões. `authsession/login_capacity_test.go:11`, `:44`. |

## RED/GREEN executados

Todos os comandos usaram `PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0` e o worktree acima. Os RED falharam por comportamento observável, não por erro de compilação. Antes do primeiro RED HTTP, o embed manager/.output/public estava ausente; coordenador criou o placeholder necessário, sem edição de manager por este implementer.

1. **1.1** — `go test ./internal/httpapi -run 'TestSessionAccount' -count=1`: RED mostrou GET 200/POST 201 para conta removida ou demoted e `TestSessionAccountLookupFailsBeforePrivateHandler` mostrou status 200 + uma leitura quando esperado 500 + zero. GREEN: `go test ./internal/httpapi ./internal/httpapi/core ./internal/auth -run 'TestSessionAccount|TestAuthenticate' -count=1` passou HTTP/core (auth sem teste correspondente ao filtro; auth completo foi executado no gate final).
2. **1.2** — `go test ./internal/httpapi -run 'TestJSONRequestMustEnd|TestIdempotencyRegistered|TestIdempotencyConcrete' -count=1`: RED incluiu PUT/PATCH sem replay, divergência 200, in-progress 200, setters 4; canais/grupos distintos 200. Método corrigido: `go test ./internal/httpapi -run 'TestIdempotencyRegistered' -count=1` passou. Parâmetros corrigidos: `go test ./internal/httpapi -run 'TestIdempotency(Concrete|ResourceAliases|Registered)' -count=1` passou. Após ajustar as fixtures, a suite também revelou `{instance}` indevidamente incluído no hash das mensagens; `TestAliasReplayUsesCanonicalUUIDAndCurrentAccess` falhou com 422 e passou após reconhecer o parâmetro pela posição /instances/{...}.
3. **1.3** — `go test ./internal/httpapi/core -run 'TestIdempotencyReplay(Rejects|Accepts)' -count=1`: RED trouxe 202 para bodies corrompidos, 200 para status zero e 100/600 preservados. GREEN: `go test ./internal/httpapi/core -run 'TestIdempotencyReplay|TestIdempotencyStoresAndReplays5xx' -count=1` passou. Substituída a expectativa antiga `TestIdempotencyReplayReturnsStoredBytesPassthrough`, que exigia corrupção; casos válidos mantêm status/bytes literais.
4. **1.4** — primeiro RED do comando conjunto acima trouxe 202 e Enqueue 1 para segundo objeto/scalar, junk e padding excessivo, com/sem chave. GREEN: `go test ./internal/httpapi -run 'TestJSONRequestMustEnd' -count=1` passou. Complemento solicitado pelo coordenador: `go test ./internal/httpapi ./internal/httpapi/core -run 'TestJSONRequestMustEnd|TestOversizedJSONDoesNot|TestFingerprintRejectsOversized' -count=1` falhou com Acquire 1 no oversized fresh-key, 422 + consulta com chave completed/in_progress e fingerprint sem MaxBytesError. Após retirar o fallback JSON, o mesmo comando passou (HTTP 0.080s/core 0.010s). O antigo teste que exigia route-only fallback em JSON foi substituído; fallback/limite multipart permanecem próprios.
5. **1.5** — `go test ./internal/httpapi -run 'TestMultipart(Message|Status|Repeated)' -count=1`: RED capturou destinatário/caption/filename/PTT vindos da query e reordenação de fields/files recebendo replay 202. GREEN: `go test ./internal/httpapi ./internal/httpapi/core -run 'TestMultipart(Message|Status|Repeated)|TestFingerprintMultipart|TestIdempotencyMultipart' -count=1` passou. O caso adicional de type conflitante foi verificado por mutação: recolocar FormValue("type") fez `TestMultipartMediaKindIgnoresQuery` falhar com 422/zero send ou publish; restaurar PostFormValue passou no focused final.
6. **1.6** — `go test ./internal/httpapi/authsession -run 'TestLoginRateLimiter(Capacity|Expired)' -count=1`: RED mostrou overflow 200, oito efeitos, 1032 buckets e admissão extra após consumir vaga expirada. GREEN: `go test ./internal/httpapi/authsession -count=1` passou, com fake clock e sem sleeps.

## Verificação final

`go test ./internal/httpapi/... ./internal/auth/... -count=1` — **exit 0** após a correção adicional da fingerprint JSON. Output em `/tmp/wzap-task1-focused-final.log`:

```text
ok  wzap/internal/httpapi                 5.806s
ok  wzap/internal/httpapi/authsession     0.007s
ok  wzap/internal/httpapi/core            0.167s
ok  wzap/internal/httpapi/representation  0.007s
ok  wzap/internal/auth                   0.864s
```

Demais subpacotes HTTP: `[no test files]`. `gofmt -l internal/httpapi` sem output; `git diff --check -- internal/httpapi` exit 0.

O primeiro focused completo após a mudança de auth falhou em 22 testes antigos cujas fixtures mintavam JWT sem conta correspondente. Foram ajustadas contas nas fixtures, sem contornar o lookup: `TestAPIKeyRotateRevokeForbidden`, `TestAPIKeyRotateRevokeNotFound`, `TestInstanceReferencesResolveAtRouter`, `TestAliasReplayUsesCanonicalUUIDAndCurrentAccess`, `TestTargetedInstanceStats`, `TestUUIDReplayRechecksOwnershipBeforeCache`, `TestRenameChangesOnlyTheAlias`, `TestInstancesCreateByUserSessionEmitsOwnerAndKey`, `TestInstancesCreateWithOwnerOverride`, `TestInstancesCreateUserOverrideForbidden`, `TestInstancesCreateKeyShownOnce`, `TestInstancesListCompleteCollection`, `TestInstanceStatsAdminCountsAll`, `TestInstanceStatsUserCountsOwnOnly`, `TestQuotaNilDepsFailClosed`, `TestRBACUserCannotReachOthersInstance`, `TestRBACRandomUUIDIsNotFound`, `TestRBACListFiltersByOwner`, `TestRBACAdminAndOwnerReachInstance`, `TestRBACMediaDownload`, `TestRBACMediaUploadGating`, `TestRBACCreateGating`. Output desse estágio: `/tmp/wzap-task1-focused.log`. Bypass da quota para admin foi mantido com conta real e Keys nil; ausência de Users não autoriza mais a sessão. Todos passaram no focused final.

## Arquivos escritos

Produção:

- internal/httpapi/core/middleware.go
- internal/httpapi/server.go
- internal/httpapi/core/idempotency.go
- internal/httpapi/core/instances.go
- internal/httpapi/messages/messages.go
- internal/httpapi/statuses/status.go
- internal/httpapi/authsession/login_ratelimit.go

Novas regressões:

- internal/httpapi/session_account_test.go
- internal/httpapi/idempotency_routes_test.go
- internal/httpapi/request_integrity_test.go
- internal/httpapi/multipart_integrity_test.go
- internal/httpapi/authsession/login_capacity_test.go

Testes/fixtures existentes ajustados:

- internal/httpapi/core/idempotency_test.go
- internal/httpapi/core/middleware_test.go
- internal/httpapi/rbac_test.go
- internal/httpapi/instance_reference_test.go
- internal/httpapi/instances_listing_test.go
- internal/httpapi/instances_create_test.go
- internal/httpapi/apikeys_test.go
- internal/httpapi/quotas_test.go

Relatório: .superpowers/sdd/plan/task-1-report.md.

## Limites e notas para integração

- Nenhuma suite global Go, integração Postgres/NATS ou build manager foi executada por este implementer. Coordenador executará gates globais e Swagger freshness após reunir áreas.
- Não foram alteradas annotations Swagger, docs/specs/tasks/AGENTS, código de auth, storage ou session.
- Sessões válidas agora fazem lookup de conta por request; a indisponibilidade correspondente fecha acesso com 500. Keys globais/por instância seguem independentes.
- Hashes de rotas sem recurso concreto e multipart com campos únicos mantêm o formato anterior. Hashes corrigidos de recursos concretos/repetições diferentes podem divergir de registros antigos incorretos (422 seguro), sem repetir efeitos.
- Nenhuma fixture HTTP implementa session.Session ou storage.MessageRepository.UpdateReceipt: não foi necessária adaptação para os trabalhos paralelos anunciados pelo coordenador.
- Trabalho entregue como alteração local, sem commit/merge/deploy e sem hipóteses além dos sete bugs.

## Complemento após revisão cruzada (coordenador)

review_manager reproduziu replay 202 para data:null, data:string e data:array, incompatíveis com envelopes das rotas atuais. Adicionados os três casos em TestIdempotencyReplayRejectsCorruptStoredResponse; RED exit 1 (status 202/replay flag) em /tmp/wzap-task1-review-red.log. validReplay agora exige data objeto não-null via JSON RawMessage, sem DTO inspect/rewrite; bytes íntegros continuam literais. GREEN: `go test ./internal/httpapi/core -run 'TestIdempotencyReplay|TestIdempotencyStoresAndReplays5xx' -count=1`, exit 0, 0.015s, /tmp/wzap-task1-review-green.log. A primeira tentativa de GREEN ficou temporariamente bloqueada por assinatura OutboxStore em edição por Task 6, sem falha de comportamento; repetida quando contratos sincronizaram. Nenhuma causa nova além de A4. Revisão final e suíte global pelo coordenador pendentes.
