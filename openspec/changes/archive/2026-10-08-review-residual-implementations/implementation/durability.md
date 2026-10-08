# Task 6 — isolamento de receipts e durabilidade terminal

Escopo concluído no worktree `/home/obsidian/dev/wzap/.worktrees/review-residual-implementations`, branch `codex/review-residual-implementations`: tasks 2.6 e 2.9. Nenhuma escrita na árvore principal, dependência, migração, workflow, commit ou chamada externa WhatsApp/Chatwoot.

## Causa e implementação

- Receipts usavam somente `wa_id`: duas instâncias com o mesmo ID recebiam `delivered_at`/`read_at`, e um receipt de uma instância podia incluir IDs exclusivamente estrangeiros no evento. `ReceiptStore`/`MessageRepository.UpdateReceipt` recebem agora `instanceID`; `Receipts.Apply` encaminha a origem e o SQL exige `instance_id` + `wa_id`. Evidência: `internal/message/receipts.go:61`, `internal/storage/postgres/messages.go:291`.
- `Outbox.complete`/`fail` confirmavam estado antes de `Writer.Write`; falha de escrita era descartada e a recovery só reabria mensagens `sending`. `MarkSent`/`MarkFailed` recebem o `model.OutboxEvent` e fazem bloqueio da mensagem, atualização e INSERT de evento na mesma transação existente. INSERT/COMMIT com falha desfazem ambos. Evidência: `internal/storage/postgres/messages.go:218`, `internal/storage/postgres/events.go:44`.
- O envelope é construído uma única vez antes do loop de persistência; o resultado de Send e o `event_id` permanecem iguais em cada tentativa. Não há novo Send durante esses retries. Estado terminal correspondente confirma retry idempotente; resultado divergente retorna `storage.ErrMessageOutcomeConflict`. Evidência: `internal/message/outbox.go:369`, `internal/storage/postgres/messages.go:237`.
- Claim e recovery compartilham um mutex e registram todo o lote ativo, inclusive mensagens aguardando o lock da instância. A recovery exclui esses IDs no SQL, impedindo reenvio enquanto um worker persiste seu resultado. Evidência: `internal/message/outbox.go:164`, `internal/message/outbox.go:242`, `internal/storage/postgres/messages.go:301`.
- Fan-out usa `CommittedNotifier.NotifyCommitted` após confirmação, sem chamar `Writer.Write` de novo. Uma chamada terminal comum repetida não entrega novamente; um retry local após acknowledgement perdido confirma o resultado correspondente e notifica uma vez. `Fanout` retorna `CommittedWriter`, mantendo o wiring existente de `cmd/wzap/main.go` por inferência de tipo; não precisei editar esse arquivo. Evidência: `internal/events/relay.go:34`, `internal/webhook/worker.go:156`.

## Arquivos

Produção: `internal/message/{receipts,outbox}.go`, `internal/storage/repository.go`, `internal/storage/postgres/{messages,events}.go`, `internal/events/relay.go`, `internal/webhook/worker.go`.

Testes atualizados: `internal/message/{receipts,outbox}_test.go`, `internal/storage/postgres/messages_test.go`, `internal/webhook/worker_test.go`.

Testes novos: `internal/message/outbox_durability_test.go`, `internal/message/outbox_postgres_test.go`, `internal/storage/postgres/messages_receipts_test.go`, `internal/storage/postgres/messages_terminal_test.go`.

Não houve alteração em contrato REST, envelope v1, specs, tasks, session, instance, Chatwoot, manager ou docs. Interfaces Go internas e seus consumidores/fakes foram ajustados em conjunto.

## RED

Todos os comandos usaram `PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0`. Os testes SQL receberam explicitamente `WZAP_TEST_DATABASE_URL` para `wzap_test` em `127.0.0.1:5435`, com credenciais de desenvolvimento documentadas; `postgrestest` criou e removeu schemas exclusivos. Nunca foi usado o schema compartilhado.

1. Probe original reutilizado por overlay com caminhos do worktree: `go test -overlay=/tmp/wzap-task6-probe-overlay.json ./internal/message ./internal/storage/postgres -run '^TestReviewWave2(FinalStatusEventRecoverable|ReceiptIsolatedByInstance)$' -count=1 -v`. Receipt: **FAIL**, “one receipt changed the other instance's message with equal wa_id”. A primeira tentativa de message encontrou a janela transitória da outra task (`FakeSession` sem `AckHistorySync`); após sincronização do core, o probe terminal isolado deu **FAIL**, “terminal message has no durable status event and recovery never emits it”. Logs: `/tmp/wzap-task6-probe-red.log`, `/tmp/wzap-task6-terminal-probe-red.log`.
2. `go test ./internal/storage/postgres -run '^TestReceiptsIsolateMessagesAndEventByInstance$' -count=1 -v`: **FAIL** para delivered/read/played; alterou milestones estrangeiros, incluiu `foreign-wa-id` no evento e escreveu evento para IDs sem correspondência local. Log: `/tmp/wzap-task6-receipts-red.log`.
3. `go test ./internal/message -run '^TestOutbox(RetriesTerminalPersistenceWithoutResending|TerminalPersistenceRollsBackStateAndEvent|TerminalPersistenceCommitsOneStateAndEvent)$' -count=1 -v`: **FAIL**. Não havia retry de persistência; o SQL confirmou `sent`/`failed` apesar de CHECK rejeitando INSERT de evento ou constraint trigger deferred rejeitando COMMIT; repetições recriavam evento/fan-out. O Writer usado neste RED tinha o repositório real de eventos como inner. Log: `/tmp/wzap-task6-terminal-red.log`.
4. `go test ./internal/message -run '^TestOutbox(RetriesTerminalPersistenceWithoutResending|AcknowledgesUncertainTerminalCommitOnce|RecoveryExcludesActiveClaimDuringTerminalRetry)$' -count=1 -v`: **FAIL**. Ack perdido não foi confirmado/notificado; a recovery não excluiu o claim ativo. Log: `/tmp/wzap-task6-retry-red.log`.
5. `go test ./internal/webhook -run '^TestFanoutCommittedEventSkipsSecondOutboxWrite$' -count=1 -v`: **FAIL**, fan-out ainda não suportava evento já gravado pela transação terminal. Log: `/tmp/wzap-task6-fanout-red.log`.

## GREEN e cobertura

- Receipt: testes SQL com duas instâncias, `wa_id` compartilhado, ID só estrangeiro e ID ausente; delivered/read/played; timestamp preservado; somente ID local no subject/envelope da instância correta; receipt sem correspondência não escreve evento.
- Unit: uma falha de persistência para sent/failed seguida de sucesso; envelope e ID idênticos nas duas tentativas; uma única chamada Send; nenhum fan-out anterior à confirmação; acknowledgement perdido confirmado uma vez; claim ativo excluído da recovery durante retry.
- SQL real: INSERT e COMMIT rejeitados para sent/failed deixam mensagem sending sem resultado terminal nem evento; sucesso produz estado e evento juntos; repetição antes/depois da publicação não recria evento/fan-out; resultados terminais divergentes são rejeitados; a recovery recupera o stale inativo e mantém o claim ativo em sending.
- Commit incerto com Postgres real: wrapper substitui somente o acknowledgement após o commit real. O relay publica/remove a row antes do retry; o retry mantém o ID original, confirma o resultado, não reinserta e faz um fan-out com uma chamada Send para sent/failed. Evidência: `internal/message/outbox_postgres_test.go:186`.
- Webhook: notificação do evento confirmado não invoca inner Writer e despacha o mesmo event/instance ID; testes anteriores de fan-out continuam válidos.

Comandos finais e resultados:

```text
go test ./internal/message ./internal/events ./internal/storage/postgres ./internal/webhook -count=1
ok wzap/internal/message          7.679s
ok wzap/internal/events           0.098s
ok wzap/internal/storage/postgres 119.199s
ok wzap/internal/webhook          1.804s

go test -race ./internal/message ./internal/webhook -count=1
ok wzap/internal/message          11.787s
ok wzap/internal/webhook          7.765s

go test -overlay=/tmp/wzap-task6-green-probe-overlay.json ./internal/message ./internal/storage/postgres -run '^TestReviewWave2(FinalStatusEventRecoverable|ReceiptIsolatedByInstance)$' -count=1 -v
PASS terminal probe; message 0.007s
PASS receipt probe; postgres 0.494s
```

DB habilitado nos três comandos finais. O probe de receipt foi copiado para `/tmp` e adaptado somente à assinatura com `instanceID`; os originais permaneceram intactos. Logs: `/tmp/wzap-task6-focused-green.log`, `/tmp/wzap-task6-race-green.log`, `/tmp/wzap-task6-probe-green.log`. O teste opcional de publicação NATS não foi habilitado nesta task; nenhum broker/stream externo foi usado. A suite global e os gates globais ficam com o coordenador.

`gofmt -l` não retornou arquivos e `git diff --check` passou após o patch final. O ambiente confirmou `WZAP_TEST_NATS_URL` ausente.

## Limites

- Não há promessa de exactly-once do transporte: crash/cancelamento antes da confirmação pode deixar um send externo já ocorrido com estado sending; um processo posterior poderá recuperá-lo. O resultado e o envelope permanecem em memória durante retries da execução atual, até seu cancelamento.
- O lock da instância permanece retido durante persistência terminal, preservando ordem e impedindo avanço de envios daquela instância enquanto o banco falha. Isso aplica backpressure e pode ocupar workers até recuperação ou shutdown.
- Após o relay apagar a row de evento pendente, a idempotência usa estado + WA ID/erro terminal correspondente. Isso depende do protocolo novo e da réplica única suportada. Não há backfill dos estados terminais legados que a versão anterior já deixou sem evento, indistinguíveis de eventos publicados.
- Webhooks continuam best-effort: queue cheia ou parada após commit pode descartar notificação; o evento durável permanece no fluxo do relay.
