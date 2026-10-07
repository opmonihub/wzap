# Tasks — correct-observed-log-errors (contrato de escopo)

> Detalhe de implementação vive em `plan.md` (referenciado por ID). Verificar
> cada item com o comando ou comportamento observável indicado.

## 1. Broker local (T1)

- [x] 1.1 Apontar o store do JetStream para o volume em `docker-compose.yml` e `docker-compose.dev.yml` — verificar com `docker compose config` mostrando `-sd` `/data` e, após recriar o broker, log sem `Temporary storage directory` e `Store Directory` em `/data`.

## 2. Foto de grupo (T2)

- [x] 2.1 Classificar corpo que não é imagem como entrada inválida na sessão, sem o envelope transitório — verificar com `go test ./internal/session/whatsmeow/ -run TestSetGroupPhoto -count=1`.
- [x] 2.2 Responder `422` no `PUT .../groups/{group_id}/photo` e deixar a foto inalterada — verificar com `go test ./internal/httpapi/ -run TestSetGroupPhoto -count=1`.

## 3. Perfil próprio (T3)

- [x] 3.1 Consultar recado e foto pelo JID de usuário, sem sufixo de dispositivo, e devolver o push name quando o upstream não informa recado ou foto — verificar com `go test ./internal/session/whatsmeow/ -run TestGetProfile -count=1`.
- [x] 3.2 Responder `200` no `GET .../profile` conectado e manter `409` desconectado — verificar com `go test ./internal/httpapi/ -run TestGetProfile -count=1`.

## 4. Console de grupos (T4)

- [x] 4.1 Impedir a busca com JID vazio ou só com espaço, sem chamar a API — verificar com o teste do `GroupDetail` (ou o comando de teste já usado em `manager/`) cobrindo o campo vazio e zero requests.

## 5. Gates (T5)

- [x] 5.1 Rodar os gates do serviço — verificar com `gofmt -l .` vazio, `go vet ./...`, `golangci-lint run` e `go test ./... -count=1`.
