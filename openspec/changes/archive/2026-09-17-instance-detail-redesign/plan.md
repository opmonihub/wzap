# Plano de execução — instance-detail-redesign

> **Para workers agênticos:** SUB-SKILL OBRIGATÓRIA: usar `openspec-apply-change` com `subagent-driven-development` (recomendado) ou `executing-plans`. Executar lote a lote, na ordem abaixo. Marcar cada item de `tasks.md` como `- [x]` somente após a verificação do lote passar.

**Objetivo:** reestruturar `manager/app/pages/instances/[id].vue` em cabeçalho + faixa de stats + navegação por 7 seções em toolbar, expondo todas as rotas instance-scoped já existentes no backend, sem nenhuma mudança em Go.

**Arquitetura:** tipos primeiro, depois 5 composables com client de sessão (`useApi().api` para JSON, `raw` para 204/binário), depois componentes folha sobre passthroughs `:ui` verbatim do template, depois fiação da página preservando emits e o contrato CSS-gated do aside/slideover, por fim i18n EN-only.

**Tech stack:** Nuxt 4 + Vue 3 + Nuxt UI 4 + zod + client de sessão; backend Go 1.26 intocado.

**Spec:** `openspec/changes/instance-detail-redesign/specs/wzap-manager/spec.md` (design: `design.md`, contrato de escopo: `tasks.md`, proposta: `proposal.md`).

## Restrições globais

- Imports internos rooted em `wzap`; Go 1.26 pinado em `go.mod`. Nenhuma mudança em Go, rotas, auth/RBAC, migration, NATS ou contrato de eventos neste change.
- Envelopes REST preservados (`{"data": ...}` / `{"error": {"code", "message"}}` + `X-Request-Id`); sem **BREAKING**.
- Console usa só cookie de sessão (`useApi().api` JSON, `raw` para 204/binário); nunca anexar header `apikey:` no manager.
- Copiar passthroughs `:ui` do template verbatim; theming só via tokens (`text-muted`, `text-highlighted`, `bg-elevated`); nenhuma variante inventada.
- Manter `data-testid`s estáveis (`pairing-card`, `webhook-card`, `test-send-card`, `messages-card`) e emits (`paired`/`updated`/`sent`/`settled` → `messagesRefresh`).
- i18n só EN sob `instances.*`; sem literais em templates; mensagens 422 do servidor sobem verbatim.
- Toda ação que toca sessão é gateada em `status === 'connected'` com aviso `notConnected`; uploads de foto são PUT octet-stream `Content-Type: image/*`; download de mídia via `raw()` → blob URL.
- Nunca logar telefones, tokens ou chaves; token Chatwoot write-only (GET mascara); pair-phone honra `expires_at`.
- NÃO colocar `v-if` no aside/slideover de mensagens e NÃO levantar seu fetch (contrato CSS-gated `hidden lg:` mantido; um fetch redundante no mobile é aceito e documentado).
- Limites: grupo nome ≤25 runes; perfil nome 1..100, recado ≤500; status texto/caption 1..700, kind image|video; privacy allowlists (`all|contacts|contact_blacklist|none`, receipts `all|none`); patches com ao menos um campo; reaction com emoji vazio remove; `Idempotency-Key` fresco (`crypto.randomUUID()`) por clique.
- Grupo criado com 201 e `invite_code` vazio reconcilia via GET invite, nunca retenta create (evita duplicar o grupo).
- Ordem dos quality gates: `pnpm --dir manager typecheck` → `pnpm --dir manager lint` → `pnpm --dir manager build` (`nuxt generate`) → `gofmt -l .` vazio → `go vet ./...` → `go test ./... -count=1` unit-only.

---

## Mapa tasks.md → arquivos → dependências

| Task | Arquivos | Depende de | Entrega testável |
|------|----------|------------|------------------|
| 1.1 Types | Modify `manager/app/types/api.ts` | — (lê `internal/httpapi/*.go` como contrato) | Novos exports compilam |
| 1.2 i18n | Modify `manager/i18n/locales/en.json` | 1.1 (nomes p/ cópia) | `lint` passa |
| 2.1 `useInstanceMessaging` | Create `manager/app/composables/useInstanceMessaging.ts` | 1.1 | typecheck passa |
| 2.2 `useInstanceGroups` | Create `manager/app/composables/useInstanceGroups.ts` | 1.1 | typecheck passa |
| 2.3 `useInstanceChannels` | Create `manager/app/composables/useInstanceChannels.ts` | 1.1 | typecheck passa |
| 2.4 `useInstanceProfile` | Create `manager/app/composables/useInstanceProfile.ts` | 1.1 | typecheck passa |
| 2.5 `useInstanceChatwoot` | Create `manager/app/composables/useInstanceChatwoot.ts` | 1.1 | typecheck passa |
| 3.1 Header+pair | Create `.../instances/InstanceHeaderStats.vue`, `PairPhoneCard.vue` | 1.1, 1.2, 2.4 | `build` renderiza |
| 3.2 Messages split | Create `ConversationList.vue`, `MessageDetail.vue`, `MessageComposer.vue` | 1.1, 1.2, 2.1 | `build` compila |
| 3.3 Groups+channels | Create `MessageActionsCard.vue`, `GroupDetail.vue`, `ChannelsCard.vue` | 1.1, 1.2, 2.1–2.3 | `build` compila |
| 3.4 Profile+integr. | Create `ProfileCard.vue`, `PrivacyCard.vue`, `DeviceActionsCard.vue`, `ChatwootCard.vue` | 1.1, 1.2, 2.4, 2.5 | `build` compila |
| 4.1 Page wiring | Modify `manager/app/pages/instances/[id].vue` | 3.1–3.4 | typecheck+build passam |
| 4.2 Delegação | Modify `TestSendCard.vue`, `MessagesCard.vue` (delegar, sem churn) | 4.1 | refresh de histórico preservado |
| 5.1 Gates front | — (verificação) | 4.2 | typecheck+lint+build |
| 5.2 Gates Go | — (verificação) | 4.2 | gofmt/vet/test unit |
| 5.3 Matriz manual | — (verificação, registra em `verify.md` no apply) | 5.1–5.2 | matriz 7 abas coberta |

Docs a consultar por task: `internal/httpapi/server.go` (inventário de rotas), `internal/httpapi/{messages,groups,newsletters,status,calls,profile,presence,pair_phone,chatwoot,lifecycle,media}.go` (limites de validação), padrões do template upstream (`settings.vue`, `inbox.vue`, `HomeStats.vue`, `customers.vue`, `settings/members.vue`).

---

## Ordem de execução (5 lotes, estritamente sequencial entre lotes)

### Lote A — Fundações (tasks 1.1, 1.2). Sequencial.

1. **1.1** Estender `manager/app/types/api.ts` (Group, Newsletter, OwnStatus, Profile, Privacy, PairPhoneResult, Chatwoot + inputs rich-send/presence/revoke/mark-read/reject-call). Commit: `feat(manager): extend instance detail API types`.
2. **1.2** Adicionar chaves `instances.*` em `manager/i18n/locales/en.json` (sections/stats/pairPhone/groups/channels/profile/privacy/chatwoot/actions), sem remover chaves irmãs. Commit: `feat(manager): add instance detail i18n copy`.

### Lote B — Composables (tasks 2.1–2.5). Paralelizável por subagente após o Lote A.

Cada task cria um arquivo em `manager/app/composables/`, espelhando estilo `useMessages.ts` + `ApiError`, sem compartilhar helper de idempotência (cada send minta chave própria):

3. **2.1** `useInstanceMessaging.ts` (location/contact/rich, revoke, mark-read, presence, download via `raw()` blob URL). Commit: `feat(manager): add instance messaging composable`.
4. **2.2** `useInstanceGroups.ts` (8 rotas + foto + invite lifecycle; regra reconcile-GET-nunca-retry). Commit: `feat(manager): add instance groups composable`.
5. **2.3** `useInstanceChannels.ts` (newsletters + status; regra 1..700 e image|video; publish fire-and-forget). Commit: `feat(manager): add instance channels composable`.
6. **2.4** `useInstanceProfile.ts` (profile/privacy/pair-phone/reject-call; aviso 501). Commit: `feat(manager): add instance profile composable`.
7. **2.5** `useInstanceChatwoot.ts` (get/put/import/command + detecção `chatwoot_disabled` 400, token mascarado). Commit: `feat(manager): add instance chatwoot composable`.

### Lote C — Componentes folha (tasks 3.1–3.4). Paralelizável por subagente após o Lote B.

8. **3.1** `InstanceHeaderStats.vue` (props-only, 4x `UPageCard subtle` com `:ui` verbatim) + `PairPhoneCard.vue` (ordenação Connect-first, código + expiry). Commit: `feat(manager): add header stats and pair-phone cards`.
9. **3.2** `ConversationList.vue` (rows selecionáveis compartilhadas) + `MessageDetail.vue` (padrão InboxMail) + `MessageComposer.vue` (footer reply card, começa com text/location; poll/reaction/list/buttons estendem no mesmo arquivo antes do Lote D). Commit: `feat(manager): add messages split leaves`.
10. **3.3** `MessageActionsCard.vue` (revoke/mark-read/presence) + `GroupDetail.vue` (lookup/create/join/detail + reconcile) + `ChannelsCard.vue` (newsletters + statuses). Commit: `feat(manager): add message actions, group and channels cards`.
11. **3.4** `ProfileCard.vue` + `PrivacyCard.vue` + `DeviceActionsCard.vue` (presence + reject-call 501) + `ChatwootCard.vue` (config/import/command + webhook URL computada). Preservar `data-testid`s. Commit: `feat(manager): add profile and integrations cards`.

### Lote D — Fiação da página (tasks 4.1, 4.2). Sequencial, após o Lote C.

12. **4.1** Reestruturar `[id].vue`: header + stats + `UDashboardToolbar` com `UNavigationMenu` de 7 seções; mover (não reescrever) info/`PairingCard`/rename → overview, `WebhookCard` → integrations, key card + delete → settings; preservar load/notFound/failure/skeleton, `deleteOpen`, `onPaired`, `onWebhookUpdated`, `messagesRefresh`, admin-gating e contrato CSS-gated do aside/slideover. Commit: `feat(manager): rework instance detail into sectioned layout`.
13. **4.2** Delegar `TestSendCard`/`MessagesCard` aos novos leaves sem mudar emits (`paired`/`updated`/`sent`/`settled`) nem `data-testid`s; em dúvida, preferir duplicação a churn de contrato. Commit: `feat(manager): delegate detail cards without contract churn`.

### Lote E — Verificação (tasks 5.1–5.3). Após o Lote D, sem código novo.

14. **5.1–5.3** Rodar gates + matriz manual (detalhe em "Verificação por lote" abaixo). `verify.md` + `retrospective.md` são produzidos no apply, ANTES do PR; arquivar por último com `openspec-archive-change`.

---

## Estratégia de worktree

- **Um único worktree isolado** para o change inteiro (motivo: todas as tasks aterrissam na mesma página e nos mesmos 3 gates de build; worktrees paralelos colidiriam em `[id].vue` e nos leaves compartilhados).
- Criar no momento do apply (não antecipar neste plano):
  ```bash
  git worktree add .worktrees/instance-detail-redesign -b feat/instance-detail-redesign
  ```
- Base: branch atual do repo no momento do apply; se `refactor/manager-page-componentization` já tiver mergeado, usar `main`.
- Execução com subagentes: **1 subagente por task dentro do lote** (lotes B e C paralelizam; lotes A, D, E são sequenciais). Revisão humana/agêntica **entre lotes**, nunca no meio de um lote.
- Commits por task (mensagens listadas acima, padrão `feat(manager): ...`); nunca `--amend`, nunca force-push, nunca commit de segredos/`.env`.
- Sem push e sem PR até o Lote E estar verde + `verify.md` + `retrospective.md` escritos.
- Detalhe de implementação (assinaturas, snippets, `:ui` verbatim) vive no design + tasks + specs; este `plan.md` é o contrato de **ordem e verificação**, não duplica código.

## Verificação por lote (comandos exatos, ordem fixa)

### Após Lote A
```bash
pnpm --dir manager typecheck
pnpm --dir manager lint
```
Esperado: ambos exit 0. Se falhar, corrigir no lote antes de avançar.

### Após cada task do Lote B
```bash
pnpm --dir manager typecheck
```
Esperado: exit 0. Falha típica: import de tipo com nome divergente de 1.1 → corrigir o nome, não o contrato Go.

### Após cada task do Lote C
```bash
pnpm --dir manager build
```
(`nuxt generate`.) Esperado: completa sem erro de template. Falha típica: `:ui` inventado ou literal fora do i18n → copiar passthrough verbatim / mover para `en.json`.

### Após Lote D (tasks 4.1 e 4.2)
```bash
pnpm --dir manager typecheck && pnpm --dir manager build
```
Esperado: ambos passam; spot-check funcional: send text → `sent` + `settled` incrementam `messagesRefresh` → linha aparece no histórico. Não levantar fetch nem adicionar `v-if` no aside/slideover.

### Lote E — gates finais (tasks 5.1–5.3)
```bash
pnpm --dir manager typecheck && pnpm --dir manager lint && pnpm --dir manager build
gofmt -l .
go vet ./...
go test ./... -count=1
```
- `gofmt -l .` deve imprimir **nada** (vazio).
- `go test` é **unit-only**: NÃO alegar integração Postgres (`WZAP_TEST_DATABASE_URL`) nem NATS (`WZAP_TEST_NATS_URL`) salvo se as variáveis estiverem setadas e alcançáveis.
- Matriz manual 5.3 (registrar em `verify.md` no apply, nunca aqui): conectado + desconectado × 7 abas (happy-path + display 409/422) × mobile slideover + desktop × light/dark × admin vs user (aba de chave oculta) × Chatwoot habilitado vs desabilitado. Depois escrever `retrospective.md` ANTES do PR (evidência primeiro, §0 Evidence + 6 seções) e arquivar com `openspec-archive-change`.

## Critérios de pronto

- [ ] Todos os 15 itens de `tasks.md` marcados `- [x]` somente após sua verificação de lote.
- [ ] Gates do Lote E verdes + `gofmt` vazio.
- [ ] `verify.md` (7 checks) + `retrospective.md` escritos antes do PR.
- [ ] Nenhuma mudança em Go, rotas, auth/RBAC, migrations, NATS ou `openspec/specs/`; `git status` mostra só arquivos do manager + artefatos openspec.
