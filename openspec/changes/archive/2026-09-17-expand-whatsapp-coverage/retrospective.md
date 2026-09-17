# Retrospective — expand-whatsapp-coverage

## §0 Evidence

- Branch `feat/expand-whatsapp-coverage`, fork point `9c3da60` (base confirmada `main`:
  `git merge-base HEAD main` = fork point), 21 commits, 106 arquivos (+23983/−1445) vs `main`.
- Gates 2026-09-17: `gofmt -l` vazio, `go vet` limpo, `golangci-lint` 0 issues (v2.13.2),
  `go test ./...` todos os pacotes ok / 0 falhas, `go build` ok.
  Integração **executada** (não skipped): Postgres
  (`WZAP_TEST_DATABASE_URL` setada, `internal/storage/postgres` 35.9s ok) e NATS
  (`WZAP_TEST_NATS_URL` setada, broker TCP `127.0.0.1:4222` OK, `internal/events` ok).
- Focados por tarefa (seção 2 do `verify.md`) todos verdes: `TestRevoke|TestMarkRead`,
  `TestPresence`, `TestPairPhone`, `TestSwagger`, `TestGroup`, `TestNewsletter`,
  `TestStatus`, `TestCall`, `TestProfile|TestPrivacy`, mais os pacotes
  `message`/`events`/`app`/`storage`.
- Onda de refino antes do fechamento (sem churn posterior de código):
  `711cb33` (lint: drop embedded field de seletores group info),
  `16ac47d` (sync status publish, prune stale, clarifica erros profile/privacy),
  `831b8d5` (status list/delete como registro since-boot).

## O que funcionou

- Ordem estrita do `plan.md` (lote A fase 1 → B ricas → C grupos/newsletters →
  D status/chamadas/perfil → E fechamento) manteve cada lote mergeável sozinho;
  a fase 1 não tocou na sessão (só transporte/validação/mapeamento de erro),
  então os 4 métodos já existentes viraram REST sem risco de regressão no adapter.
- Padrão de handler repetido (`instanceID()` → `denyForeignInstanceKey()` →
  `Get()` → `authorizeInstance()` → valida → serviço → envelope) copiado de
  `connection.go`/`messages.go` reduziu variação entre ~20 rotas novas.
- TDD por família com comandos de verificação declarados no `tasks.md` travou a
  matriz de erros (`404`/`403`/`409`/`422`/`501`) antes da implementação;
  os testes focados viraram a evidência direta do `verify.md`.
- Migration só aditiva (`00006_group_newsletter.sql`, `CREATE TABLE IF NOT EXISTS`,
  schema isolado por teste via `postgrestest`) aplicou em banco limpo sem tocar
  no schema compartilhado — zero atrito com os testes Postgres.

## O que atrasou / desviou

- Escopo grande (8 capabilities novas + 4 modificadas, 17 tarefas) numa única
  branch: o diff vs `main` passou de 100 arquivos, o que torna a revisão final
  mais cara do que lotes A–D teriam sido como PRs separados.
- `go` fora do `PATH` no shell não-interativo (só em `/usr/local/go/bin`):
  primeiro gate falhou com `command not found` até o `PATH` ser ajustado —
  mesmo tropeço já registrado na retrospectiva do `canonical-zerolog-logger`.
- Toolchain do ambiente (`go1.27.0`) ≠ pin do `go.mod` (`go 1.26.0`): gates
  passaram, mas a divergência precisa ser registrada no `verify.md` (feito, §1).
- Ajustes finos consumiram a reta final (limite de nome de grupo em runes,
  grupo parcial em falha de convite, semântica sync-`202` do status, `updated_at`
  de metadados): corretos, mas sinal de que allowlists/limites e formato de
  `revoked:false` mereciam Tavola de decisão mais cedo no apply.

## Dívidas assumidas (v2 ou changes futuros)

- Manager web (`/manager`): telas de QR por código, grupos, enquetes, status e
  privacidade seguem fora de escopo — change dedicada de frontend.
- Espelho Chatwoot dos novos tipos inbound (votos, reações, listas, grupos,
  chamadas) e importação de histórico cobrindo os novos tipos: changes dedicadas
  do conector.
- Open questions do design viraram decisões documentadas no apply (`501` para
  não suportado pelo upstream, remoção de reação via emoji vazio, `revoked:false`
  + motivo, status publish `202` fire-and-forget sem retry de outbox); se o
  upstream mudar o suporte, revisar o mapeamento por operação no Swagger.
- Rate limits de msg/min e req/min e multi-réplica seguem fora de escopo
  (locks process-local, 1 réplica).

## Sinais para a próxima

- Exportar `PATH` com `/usr/local/go/bin` no início de qualquer sessão de gates
  (segunda change seguida com o mesmo tropeço — virar checklist de abertura).
- Registrar toolchain real vs. pin do `go.mod` no `verify.md` desde o primeiro
  apply (padrão herdado do `canonical-zerolog-logger`, reutilizado aqui).
- Changes com 4+ famílias: preferir PRs por lote (A, B, C, D) em vez de uma
  branch única — revisão de 100+ arquivos não escala.
- Decisões de mapeamento de erro do upstream (`501` vs `422`, janelas, formatos
  de `false` + motivo) merecem confirmação contra o adapter no primeiro dia do
  lote, não na véspera do fechamento.

## Decisões que valem reuso

- Fase 1 sem tocar na sessão + fase 2 estendendo `Session`/adapter/fakes em
  ordem de dependência: blueprint copiável para qualquer "expor o que já existe,
  depois expandir".
- Ricas assíncronas no outbox (`202` + idempotência + `X-Idempotent-Replay`) vs.
  resto síncrono, com a exceção do status (`202` sobre backing síncrono
  fire-and-forget) registrada como ruling do controller — resolveu o dilema
  sync/async sem rework de fila.
- Figurinha como `type=sticker` no `/messages/media` existente (validação webp +
  `MaxMediaBytes`) em vez de endpoint novo: reuso que eliminou uma superfície
  inteira de validação/idempotência/docs.
- Grupos/newsletters com metadados em cache aditivo + `updated_at` exposto e
  refresh sob demanda: padrão para qualquer dado cuja fonte de verdade é o
  WhatsApp.

## Riscos residuais

- Metadados de grupo/newsletter podem divergir do upstream entre refreshes —
  mitigado com `updated_at` exposto e nunca apresentado como fonte de verdade;
  acompanhar reclamações de stale no consumo.
- Presença sem modo contínuo/heartbeat por design (risco de throttle/ban) —
  se consumidores fizerem polling, o abuso aparece no upstream antes de aparecer
  nos nossos logs; documentar uso pontual não basta, monitorar volume.
- Eventos novos (votos, reações, respostas, grupo, chamada, status) são opt-in
  no `events` do webhook sem mudar o default — consumidores que não optarem
  simplesmente não os verão; risco de "silêncio percebido como bug" no suporte.
- `pair-phone` exige `connect` prévio (`409` sem canal): fluxo em duas etapas
  que precisa estar no guia do consumidor, senão vira ticket recorrente.
