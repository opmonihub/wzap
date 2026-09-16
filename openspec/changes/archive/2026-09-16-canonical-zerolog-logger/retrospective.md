# Retrospective — canonical-zerolog-logger

## §0 Evidence

- Branch `sdd/canonical-zerolog-logger`, fork point `d80b939`, 17 commits, 86 arquivos
  (+2136/−1106). Base confirmada `main` (`git merge-base main HEAD` = fork point).
- Gates 2026-09-16: `go vet` limpo, `golangci-lint` 0 issues (v2.13.2),
  `go test ./...` 27 pacotes ok / 0 falhas, `go build` ok (+ `CGO_ENABLED=0`),
  `gofmt -l` vazio. Integração Postgres/NATS skipped (envs ausentes).
- `TestNoSlogInProd` PASS; grep `log/slog` em prod → zero.
- Wave de revisão final (`b1e731e`) antes do fechamento: PII, nível único,
  skip no manager, testes race-safe — sem churn posterior de código.

## O que funcionou

- Fábrica única primeiro (`internal/logger` + `NewTestLogger`/`AssertNoSecret`):
  as 10 tarefas de migração viraram aplicação mecânica do padrão universal do
  `plan.md`, sem redesenho no meio.
- `slogBridge` temporário com gate de deleção explícito (Task 4.1) manteve
  `main` compilando a cada commit intermediário — nenhum commit quebrado na série.
- Gates por tarefa (`vet` + `gofmt` + focado + full suite antes de cada commit)
  impediram regressão silenciosa; o throttle (relay/outbox/webhook) e o relevel
  (chatwoot) vieram com testes de contagem que travaram os invariantes
  (dead-letter == 1, warn cap 1+8).
- Smoke `json|console|text` + rejeições via `logger.New` validou o contrato de
  `WZAP_LOG_*` sem precisar subir o binário completo.

## O que atrasou / desviou

- O `plan.md` (585 linhas) repetia código já executado nas Tasks 1–2.1 como
  "Status: DONE" em vez de referenciar os commits — leitura mais lenta para quem
  retoma no meio; handoff funcionou pelos commits, não pelo plano.
- Toolchain do ambiente (`go1.27.0`) ≠ pin do `go.mod` (`1.26.0`): gates passaram,
  mas a divergência não foi registrada em nenhum artefato até este fechamento.
- `go` fora do `PATH` no shell não-interativo (só em `/usr/local/go/bin`):
  primeiro gate falhou com `command not found` até o `PATH` ser ajustado.

## Dívidas assumidas (v2 ou changes futuros)

- Token do Chatwoot segue em claro (write-only) — pré-existente, fora de escopo.
- Integração Postgres/NATS não executada neste fechamento (envs ausentes);
  reexecutar com as URLs no pós-merge ou no CI.
- Throttle `warnEvery` default 1min é fixo por struct — sem tuning via env;
  aceitar até houver sinal de flood residual em produção.

## Sinais para a próxima

- Registrar toolchain real vs. pin do `go.mod` no `verify.md` desde o primeiro
  apply (virou item padrão a partir deste).
- Exportar `PATH` com `/usr/local/go/bin` no início de qualquer sessão de gates.
- Planos longos: seção "Status" deve apontar commits, não duplicar código de
  tarefas concluídas.

## Decisões que valem reuso

- Padrão universal de tradução (slog k-v → zerolog encadeado, `uuid`→`Str`,
  `error`→`Err`, ctx fora dos logs) como bloco copiável no plano — reduziu
  variação entre 10 autores-tarefa a quase zero.
- `TestNoSlogInProd` (walk hermético via `runtime.Caller`) como grep gate
  executável: barato (<1s), impossível de esquecer no CI futuro.
- Helpers de teste canônicos (`NewTestLogger` + `AssertNoSecret`) eliminaram três
  handlers in-memory ad-hoc e unificaram os guards anti-segredo num só lugar.

## Riscos residuais

- Relevel `Warn→Debug` no espelho Chatwoot pode ocultar falhas reais se o filtro
  `reason` não for monitorado — acompanhar volume de `Debug` com `reason` nas
  primeiras semanas.
- Alias `text` com deprecation `Warn` única: consumidores que parseiam logs em
  `text` veem uma linha extra no boot — intencional e documentado no README.
