# Retrospective — instance-detail-redesign

Date: 2026-09-17. Branch: `main` (head `ceb595b`, base `58b60d9`).
Scope: rework frontend-only do detalhe da instância (header + stats + 7 seções,
cobertura total das rotas instance-scoped existentes); zero arquivos Go.

## §0 Evidence

- `tasks.md`: 14/15 itens `[x]` (1.1–5.2); 5.3 (matriz manual) segue `[ ]`
  por exigir backend vivo/browser — listado em `verify.md` §4.
- `verify.md` §§1–7: `pnpm --dir manager typecheck` PASS; `lint` PASS
  (0 errors, 2 warnings pré-existentes fora do escopo); `build` PASS
  (7 rotas prerenderizadas); `gofmt -l .` vazio; `go vet ./...` PASS;
  `go test ./... -count=1` PASS (unit-only, sem alegação Postgres/NATS);
  diff `58b60d9..ceb595b` = 20 arquivos, só `manager/`.
- Code review (subagente, 2026-09-17): **Ready with notes** — 0 Critical,
  1 Important (Chatwoot desconhecido vs desabilitado), 5 minors.
- NÃO EXECUTADO: matriz manual 5.3 (7 abas × conectado/desconectado ×
  mobile/desktop × light/dark × admin/user × Chatwoot on/off).

## 1. What went well

- Ordem types → composables → leaves → page wiring → i18n funcionou: cada
  lote compilava isolado (typecheck por composable, build por leaf), então a
  fiação da página (Lote D) não precisou retrabalhar contratos.
- Disciplina de contratos: emits (`paired`/`updated`/`sent`/`settled` →
  `messagesRefresh`), `data-testid`s e i18n EN-only sob `instances.*` ficaram
  estáveis; o revisor não encontrou nenhuma quebra de contrato, auth ou
  idempotência.
- Regras de borda do backend espelhadas com precisão (rune-count, allowlists,
  reconcile-GET-nunca-retry no group-create, fire-and-forget no status,
  token Chatwoot write-only) — o review elogiou explicitamente esses pontos.
- Gates verdes de primeira nesta sessão de fechamento: nenhum fix de código
  foi necessário para typecheck/lint/build/gofmt/vet/test.

## 2. What didn't go as planned

- O trabalho aterrissou direto em `main` (commits `8a24727..ceb595b`), não
  num worktree/branch isolado como o `plan.md` mandava (`.worktrees/
  instance-detail-redesign` + `feat/instance-detail-redesign`). Sem isolamento,
  sem revisão entre lotes, sem base confirmada — o plano de worktree existia
  mas não foi seguido.
- Os artefatos openspec do change (proposal/design/plan/tasks/specs) nunca
  foram commitados — estão untracked até agora. O histórico conta a
  implementação, mas não o planejamento.
- O estado "Chatwoot desconhecido" vazou para a UI como "Disabled"
  (`chatwootState` inicia `false`): informação factualmente errada no
  primeiro paint, capturada só no review final em vez de num gate.
- Um literal hardcoded (`statusKinds` em `ChannelsCard.vue:28-31`) passou
  pelos gates — lint/typecheck/build não pegam brecha i18n; só o olho do
  revisor.

## 3. Decisions worth keeping

- `tasks.md` como contrato de escopo + verificação por lote com comandos
  exatos: permitiu marcar 14/15 com evidência fresca em vez de "deveria passar".
- Não corrigir o Important do review nesta sessão: `tasks.md` não o contém,
  e expandir escopo no fechamento (código novo → re-gates → re-review)
  sem ordem humana seria o oposto de "finish". Registrar honestamente como
  follow-up > correção silenciosa.
- `verify.md` separando PASS estático (§§1–3) de NÃO EXECUTADO (§4) e de
  decisão de escopo (§6): o leitor sabe exatamente o que foi provado e o
  que falta antes do release.

## 4. Risks carried to release (todos gated na matriz manual 5.3)

- Header Chatwoot exibe Disabled antes do fetch (Important do review) —
  usuários podem concluir que a integração global está off.
- `PrivacyCard` permite save antes do load resolver (sobrescrita com
  all-`all`) — só testável com backend lento/falhando.
- Matriz interativa inteira não executada: pair-phone Connect-first, 409/422
  verbatim por aba, slideover mobile vs aside desktop, light/dark,
  admin vs user (chave oculta), Chatwoot enabled/disabled, import parcial.

## 5. Follow-ups proposed (out of scope for this change)

1. Estado triplo do Chatwoot no header (`undefined` → `common.notSet`;
   Enabled/Disabled só após `chatwoot-loaded`) — o Important do review.
2. Desabilitar save do `PrivacyCard` até `privacy !== null`.
3. Chaves i18n para os labels de `statusKinds` (`ChannelsCard.vue:28-31`).
4. Remover ou documentar `void props.status` (`ChatwootCard.vue:154`).
5. Desempilhar banners notice+failure no `GroupDetail` após reconcile com erro.
6. Clamp client-side em `selectable_count` (`MessageComposer.vue:170`).
7. Executar a matriz manual 5.3 e anexar resultados a `verify.md` §4.

## 6. Process notes

- `plan.md` exigia worktree único + subagentes por task + revisão entre
  lotes; nada disso aconteceu (trabalho direto em `main`). Para o próximo
  change, criar o worktree no apply — o plano já traz o comando pronto.
- Forward-pointer: se a matriz manual 5.3 ou os follow-ups revelarem
  quebras, anexar a `verify.md` §§4–5 — nunca reescrever esta retrospectiva.
- Arquivamento (`openspec-archive-change`) fica por último, após o PR —
  não nesta sessão.
