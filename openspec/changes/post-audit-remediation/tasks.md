## 1. Agregação de instância

- [x] 1.1 Commit do working tree de instance-config-aggregation + marcar 7.2 no archive tasks — gates verdes.

## 2. Webhook / docs

- [x] 2.1 Fanout após outbox OK — `go test ./internal/webhook/ -count=1`.
- [x] 2.2 README shutdown, media-migrate, notas operacionais.

## 3. OpenSpec hygiene

- [x] 3.1 Arquivar changes completas + verify onde falta (3/5 arquivadas; document-all-http-routes e whatsmeow-parity-routes bloqueadas por delta spec — follow-up).
- [x] 3.2 Reconciliar whatsmeow-parity-routes tasks vs testes (tasks [x], archive pendente).

## 4. Manager

- [x] 4.1 UI PUT default-disappearing.
- [x] 4.2 Query `section` na URL do detalhe.
- [x] 4.3 Api key localStorage + notificações placeholder.

## 5. Mensagens

- [x] 5.1 Enqueue checa sessão/resolver após status DB.

## 6. Fechamento

- [x] 6.1 Gates finais e progress ledger.
