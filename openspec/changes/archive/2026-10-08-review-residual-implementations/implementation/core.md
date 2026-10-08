# Task 2 — núcleo, segurança e import

Worktree: `/home/obsidian/dev/wzap/.worktrees/review-residual-implementations`, branch `codex/review-residual-implementations`. Escopo implementado: 2.1–2.5, 2.7–2.8, 2.10–2.11. Recibos e durabilidade terminal (2.6/2.9) pertencem ao outro writer.

## Causas e correções

| Task | Causa demonstrada | Correção e evidência principal |
| --- | --- | --- |
| 2.1 | Duas leituras de snapshots antigos sobrescreviam campos omitidos; webhook-only também chamava UpdateIdentity. | `internal/instance/service.go:331`: locker próprio de PATCH por instância, antes de Get e até o fim das gravações; UpdateIdentity somente com campos presentes. Testes com barreiras, cancelamento e liberação após erro em `update_concurrency_test.go`. |
| 2.2 | Logs incluíam campos jid; mensagens de parse/ausência/mismatch e erros upstream eram propagados ao reason. | `internal/session/whatsmeow/manager.go:318`, `errors.go:20`: operação/categoria segura com Unwrap preservando errors.Is/As; remove campos de JID em restore e sanitiza o erro antes do primeiro log de conexão. `restore_privacy_test.go` cobre parse, lookup upstream, ausência, mismatch, conexão upstream e sucesso em DB real. |
| 2.3 | Clientes http.Client seguiam redirects com os headers autenticados. | `internal/chatwoot/client/client.go:35`, `internal/webhook/ssrf.go:177`: CheckRedirect retorna ErrUseLastResponse; tratamento normal considera 3xx falha e fecha o corpo. Testes httptest verificam zero requisições no segundo origin, inclusive HTTPS → HTTP. |
| 2.4 | SealToken considerava toda entrada enc:v1: já cifrada, embora Get entregue plaintext. | `internal/chatwoot/config/token.go:23`: toda entrada não vazia é cifrada. Prefixos opacos passam round trip, exigem chave e ficam distintos no SQL, inclusive Get → Put. Empty/tamper/wrong-key continuam cobertos. |
| 2.5 | Um único IP coincidente no DNS de outro hostname liberava todos os endereços privados. | `internal/chatwoot/inbound/ssrf.go:170`: confiança apenas no hostname configurado ou no mesmo IP literal; aliases DNS não herdam confiança. A regra atua na URL, redirect e dial. Testes bloqueiam lista mista e preservam host privado configurado e representações equivalentes de IPv6/IPv4. |
| 2.7 | Snapshot seguido de Reset integral apagava chunks recebidos durante o import. | `internal/session/historysync.go:160`, `internal/chatwoot/import/run.go:94`: AckHistorySync reconhece watermark do snapshot; journal de observações posteriores reconstrói o feed, incluindo atualizações do mesmo message/contact/chat ID. Snapshots de outro feed e anteriores a Reset não apagam dados novos. Testes reais com barreiras cobrem sucesso, conta inválida e trigger SQL que rejeita mensagem. |
| 2.8 | Eventos history-sync, preview e completion sempre alimentavam o acumulador. | `internal/session/whatsmeow/manager.go:65`, `events.go`, `historysync.go`: managers/sessões são inertes por padrão; opção explícita WithHistorySync. Boot passa `cfg.Chatwoot.ImportDBURL != ""`. Fakes têm a mesma opção/default; testes enabled/disabled cobrem Create e restore. |
| 2.10 | Resultados contains eram unidos e escolhidos/fundidos sem verificar identidade. | `internal/chatwoot/contacts/contacts.go:50`: whitelist de variantes telefônicas normalizadas antes da seleção/merge; `exactIdentifier` para grupos e recuperação de 422. Testes com HTTP real impedem escolha, merge ou atualização de contatos alheios. |
| 2.11 | init buscava apenas uma sessão já existente e pareamento por número pulava Connect. | `internal/chatwoot/inbound/webhook.go:594`: reutiliza instance.Service.Connect e depois PairPhone; status omite DeviceJID, erros têm confirmações seguras e sessão conectada permanece conectada. Testes usam o Service real com storage/session fakes e verificam QR, código, estado persistido e ausência de JID. |

## Arquivos

- Instância: `internal/instance/service.go`, `update_concurrency_test.go`.
- Sessão compartilhada: `internal/session/historysync.go`, `historysync_test.go`, `session.go`.
- Fakes: `internal/session/sessiontest/fake.go`, `historysync_test.go`.
- Whatsmeow: `internal/session/whatsmeow/{manager.go,manager_test.go,errors.go,restore_privacy_test.go,events.go,historysync.go,historysync_test.go,reconnect_test.go}`. O último arquivo recebeu somente a correção de fixture descrita abaixo.
- Webhook: `internal/webhook/{deliver.go,ssrf.go,redirect_test.go}`. `worker.go` e `worker_test.go` reservados ao writer de durabilidade.
- Chatwoot: `client/{client.go,redirect_test.go}`, `config/{token.go,token_test.go}`, `contacts/{contacts.go,contacts_test.go,resolve_exact_test.go}`, `import/{run.go,run_test.go,scheduler_test.go,snapshot_concurrency_test.go}`, `inbound/{ssrf.go,ssrf_identity_test.go,webhook.go,webhook_test.go,command_lifecycle_test.go}`.
- SQL sealing: `internal/storage/postgres/chatwoot_test.go`.
- Boot: somente a chamada NewManager em `cmd/wzap/main.go:213`; arquivo liberado ao coordenador antes do wiring de durabilidade.

## RED observado antes das correções

Ambiente: `PATH=/usr/local/go/bin:$PATH`, `GOTOOLCHAIN=go1.26.0`. Execuções de DB tiveram WZAP_TEST_DATABASE_URL explicitamente configurada para `wzap_test` em `127.0.0.1:5435`, alcançável, com schemas isolados pelos helpers. Nenhum schema compartilhado foi usado. Os probes fornecidos foram reutilizados como referência e convertidos em testes legíveis/com barreiras; não houve nova busca de candidatos.

| Comando (prefixos de ambiente acima) | Resultado RED |
| --- | --- |
| `go test ./internal/chatwoot/client ./internal/webhook ./internal/chatwoot/config -run 'TestAuthenticatedClientRejectsRedirect\|TestDeliverRejectsCredentialRedirect\|TestSealTokenTreatsPrefixedInputAsPlaintext' -count=1` | Exit 1: cada redirect atingiu o segundo origin; token prefixed passou intacto e falhou na abertura. |
| `go test ./internal/storage/postgres -run TestChatwootConfigPrefixedTokenSealedAcrossGetPut -count=1` com DB | Exit 1: plaintext intacto no SQL e Get falhou na abertura do envelope. |
| `go test ./internal/instance -run 'TestServiceConcurrentPartialUpdates\|TestServiceWebhookOnlyUpdate\|TestServiceUpdateWaiting' -count=1` | Exit 1: perda de identidade/webhook, webhook-only dependendo do writer de identidade e cancelamento ignorado. |
| `go test ./internal/session/whatsmeow -run 'TestRestoreParseFailure\|TestLoadBoundDeviceHides\|TestRestoreStoredDevicePrivacy' -count=1` com DB | Exit 1: parse/lookup/ausência/mismatch/upstream/sucesso expuseram JID. |
| `go test ./internal/session/whatsmeow -run TestClassifySessionErrorPreservesUpstreamContextPrivately -count=1` | Exit 1: contexto upstream perdido e texto contendo JID propagado. |
| `go test ./internal/chatwoot/inbound -run 'TestAttachmentMixedDNS\|TestAttachmentRedirectCannotBorrow\|TestAttachmentConfiguredPrivateHost' -count=1` | Exit 1: URL, dial e redirect atingiram loopback de outro hostname. |
| `go test ./internal/chatwoot/import -run TestRunImportAcknowledgesOnlyProcessedSnapshot -count=1` com DB | Exit 1: sucesso apagou todo o chunk posterior ao snapshot. Falha já conservava o feed e permaneceu protegida. |
| `go test ./internal/session/whatsmeow ./internal/session/sessiontest -run 'TestHistorySyncDisabled\|TestFakeSessionHistorySyncDisabled' -count=1` | Exit 1: chunks/contatos/conversas/previews/completion eram retidos sem habilitação. |
| `go test ./internal/chatwoot/contacts -run 'TestResolveContains\|TestResolveIdentifier' -count=1` | Exit 1: telefone alheio escolhido/fundido e identifier parcial escolhido/atualizado. |
| `go test ./internal/chatwoot/inbound -run 'TestCommandInitUsesFresh\|TestCommandStatusHides\|TestCommandInitFailure' -count=1` | Exit 1: init/init:number não criaram sessão, status e falha de init expuseram JID. |
| `go test ./internal/chatwoot/inbound -run TestAttachmentConfiguredIPIdentityAllowsEquivalentLiterals -count=1` | Exit 1 durante revisão: endurecimento inicial recusava formas equivalentes do IP configurado; corrigido com IP.Equal apenas para dois literais. |

## GREEN e verificações

- `go test ./internal/chatwoot/client ./internal/webhook ./internal/chatwoot/config -count=1`: exit 0 após as correções de segurança.
- `go test ./internal/instance -count=1`: exit 0 após o locker/UpdateIdentity parcial.
- `go test ./internal/session/whatsmeow -run 'TestRestoreParseFailure|TestLoadBoundDeviceHides|TestRestoreStoredDevicePrivacy|TestClassifySessionError' -count=1` com DB: exit 0.
- `go test ./internal/chatwoot/contacts -count=1` e `go test ./internal/chatwoot/inbound -count=1`: exit 0 após correspondência exata/lifecycle.
- Primeira suite sessão/import encontrou duas expectativas antigas que exigiam causa literal upstream ou JID (`TestRestoreAllReflectsFailure`, `TestRestoreAllMissingDeviceReflectsErrNoDevice`). Atualizadas para operação/categoria segura; não há falha omitida.
- `go test ./internal/instance ./internal/session/... ./internal/chatwoot/... ./internal/webhook -count=1` com DB: exit 0, 13 pacotes; whatsmeow 21,979 s e import 13,297 s.
- `go test ./internal/storage/postgres -run 'TestChatwootConfig.*(Token|LegacyPlaintext)' -count=1` com DB: exit 0, última execução 15,089 s. Sealing/tamper/wrong-key/empty/prefix/Get→Put/legacy foram exercitados.
- Race inicial: exit 1, com compilação transitória dos consumers de durabilidade enquanto outro writer alterava CommittedWriter/CommittedNotifier/MarkFailed; nenhum desses arquivos foi alterado por este writer. Também falhou `TestDropAfterCompletedLoginStillRetries`.
- Diagnóstico de fixture: `go test -race ./internal/session/whatsmeow -run TestDropAfterCompletedLoginStillRetries -count=100` reproduziu duas falhas. O sleepRecorder retornava imediatamente e o reconnect falso retornava nil, permitindo concluir o retry e apagar reconnectRun antes da asserção. A fixture agora segura o backoff por canal, verifica retry registrado e libera o canal para comprovar uma tentativa. Produção e timeouts não foram relaxados; não é uma nova causa de produto. O mesmo comando passou 100 vezes, exit 0 (1,377 s).
- Verificação race final, após fontes congeladas e sincronização das interfaces: `go test -race ./internal/instance ./internal/session/... ./internal/webhook ./internal/chatwoot/client ./internal/chatwoot/config ./internal/chatwoot/contacts ./internal/chatwoot/import ./internal/chatwoot/inbound -count=1` com DB: exit 0, dez pacotes. whatsmeow 28,793 s, import 20,697 s, webhook 9,395 s; nenhum race reportado.
- `gofmt -l` nos 35 arquivos Go deste writer: saída vazia. `git diff --check` nas áreas deste writer: exit 0.

## Limites

Não foram usados WhatsApp/Chatwoot reais, NATS de produção ou schemas compartilhados. Nenhuma dependência, migração, workflow, commit, merge ou deploy foi adicionado. A suite global, vet/build/lint e gates de manager continuam com o coordenador. O locker segue a arquitetura suportada de uma réplica; o feed de import continua em memória e agora retém observações pendentes até confirmação.
