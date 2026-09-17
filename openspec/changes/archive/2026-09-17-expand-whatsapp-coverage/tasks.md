## 1. Fase 1 — expor sessão existente via REST

- [x] 1.1 Expor revogação e leitura (`revoke`, `mark-read`) sobre a sessão existente, verificando `200` conectado, `409` desconectado e `422` sem autor em grupo via `go test ./internal/httpapi/ -run 'TestRevoke|TestMarkRead' -count=1`
- [x] 1.2 Expor presença (`presence` com allowlist de estados) sobre a sessão existente, verificando `200` válido e `422` estado desconhecido via `go test ./internal/httpapi/ -run TestPresence -count=1`
- [x] 1.3 Expor pareamento por telefone (`pair-phone` exigindo `connect` prévio) sobre a sessão existente, verificando `200` com canal aberto e `409` sem canal via `go test ./internal/httpapi/ -run TestPairPhone -count=1`
- [x] 1.4 Documentar as rotas da fase 1 no Swagger com dual auth e envelopes, verificando `swag init --parseInternal` sem diff e `go test ./internal/httpapi/ -run TestSwagger -count=1`

## 2. Fase 2 — mensagens ricas

- [x] 2.1 Aceitar enquete, reação, figurinha (`type=sticker`), lista e botões no aceite `202` idempotente, verificando replay, `422` e `409` via `go test ./internal/message/ ./internal/httpapi/ -count=1`
- [x] 2.2 Publicar eventos inbound de voto, reação e resposta interativa no envelope versionado, verificando `event_id` estável e entrega NATS/webhook via `go test ./internal/events/ ./internal/app/ -count=1`

## 3. Fase 2 — grupos e newsletters

- [x] 3.1 Implementar ciclo de vida, membros/admins e convites de grupos com erros `403`/`404`/`422`, verificando cenários de dono e convite via `go test ./internal/httpapi/ -run TestGroup -count=1`
- [x] 3.2 Implementar seguir/consultar/listar newsletters com paginação por cursor, verificando `404` desconhecido e página com `next_cursor` via `go test ./internal/httpapi/ -run TestNewsletter -count=1`
- [x] 3.3 Publicar eventos de grupo no envelope versionado, verificando ator, afetado e `event_id` estável via `go test ./internal/events/ ./internal/app/ -count=1`
- [x] 3.4 Criar migration aditiva de metadados de grupo/newsletter com refresh sob demanda, verificando aplicação em banco limpo via `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' go test ./internal/storage/... -count=1`

## 4. Fase 2 — status, chamadas, perfil e privacidade

- [x] 4.1 Publicar, listar e apagar status (texto/imagem/vídeo) com limite de mídia, verificando `202`/`200` e `422` acima do limite via `go test ./internal/httpapi/ -run TestStatus -count=1`
- [x] 4.2 Publicar eventos de chamada e rejeitar a ativa quando suportado (`501` documentado senão), verificando evento e códigos via `go test ./internal/app/ ./internal/httpapi/ -run 'TestCall' -count=1`
- [x] 4.3 Consultar e atualizar perfil e privacidade com allowlists, verificando `200` válido e `422` fora da allowlist via `go test ./internal/httpapi/ -run 'TestProfile|TestPrivacy' -count=1`

## 5. Fechamento

- [x] 5.1 Atualizar README, Swagger e `openspec/specs/` com rotas, eventos e exemplos sem **BREAKING**, verificando `gofmt -l .` vazio e `go vet ./...` limpo
- [x] 5.2 Executar gates (`go vet`, `golangci-lint run`, `go test ./...`, `go build ./...`), verificando saída limpa antes de qualquer apply (detalhe de implementação em `plan.md` por ID de tarefa)

> Nota: o detalhamento de implementação (arquivos, micro-passos TDD e pontos
> de commit) vive em `plan.md`, referenciado por ID de tarefa, e será escrito
> na etapa de planejamento — não nesta proposta.
