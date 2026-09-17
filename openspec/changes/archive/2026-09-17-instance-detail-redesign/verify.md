# Verify — instance-detail-redesign

Date: 2026-09-17. Branch: `main` (head `ceb595b`). Base do change: `58b60d9`
(`docs(openspec): arquiva manager-page-componentization`).
Ambiente: repo normal (`GIT_DIR == GIT_COMMON`), sem worktree — o trabalho já
está commitado em `main`, e os artefatos openspec do change estão untracked.

Sem backend vivo ou browser neste ambiente; tudo abaixo é evidência estática
(execução fresca nesta sessão), e §4 lista o que NÃO foi executado.

## 1. Gates do manager (todos PASS)

| Gate | Comando (da raiz do repo) | Resultado |
|---|---|---|
| typecheck | `pnpm --dir manager typecheck` | PASS, exit 0, sem erros |
| lint | `pnpm --dir manager lint` | PASS, exit 0 — 0 errors, 2 warnings (pré-existentes, fora do escopo: `vue/no-required-prop-with-default` em `DataTableToolbar.vue:9` e `PageState.vue:6`) |
| build | `pnpm --dir manager build` (`nuxt generate`) | PASS, exit 0 — prerender de 7 rotas, saída estática em `manager/.output/public` ("You can now deploy .output/public to any static hosting!") |

## 2. Superfície Go intocada

- `gofmt -l .` (com `PATH=/usr/local/go/bin:$PATH`): saída vazia — limpo.
- `go vet ./...`: PASS, exit 0.
- `go test ./... -count=1`: PASS — todos os pacotes `ok` (sem
  `WZAP_TEST_DATABASE_URL`, testes de integração Postgres pulados por convenção
  do repo; sem `WZAP_TEST_NATS_URL`, sem alegação de integração NATS).
- Diff `58b60d9..ceb595b --stat`: 20 arquivos, todos sob `manager/`
  (types, 5 composables, ~12 leaves/sections, `[id].vue`, `TestSendCard`,
  `MessagesCard`, `en.json`) — zero `.go`, migrations, deps ou contrato de eventos.

## 3. Matriz estática (tasks 1.1–4.2)

- `manager/app/types/api.ts`: 44 ocorrências de Group|Newsletter|OwnStatus|
  Profile|Privacy|PairPhone|Chatwoot — tipos + inputs rich-send/presence/
  revoke/mark-read/reject-call presentes (task 1.1).
- `manager/i18n/locales/en.json`: chaves `instances.*` incluem `sections`,
  `stats`, `pairPhone`, `groups`, `channels`, `profile`, `privacy`, `chatwoot`
  (+ `send`, `messages`, `pairing`, `webhook`) — sem literais removidos de irmãs
  (task 1.2).
- Composables exportados: `useInstanceMessaging`, `useInstanceGroups`,
  `useInstanceChannels`, `useInstanceProfile`, `useInstanceChatwoot`
  (tasks 2.1–2.5); cada send minta `Idempotency-Key` fresca via
  `crypto.randomUUID()` por clique; download de mídia via `raw()` → blob URL.
- Leaves presentes e compilando via `build`: `InstanceHeaderStats`,
  `PairPhoneCard`, `ConversationList`, `MessageDetail`, `MessageComposer`
  (sub-tabs text/media/location/contact/poll/reaction/list/buttons),
  `MessageActionsCard`, `GroupDetail`, `ChannelsCard`, `ProfileCard`,
  `PrivacyCard`, `DeviceActionsCard`, `ChatwootCard` (tasks 3.1–3.4).
- `[id].vue`: 7 seções (overview, messages, groups, channels, profile,
  integrations, settings) em `UNavigationMenu` no toolbar; preservados
  load/notFound/failure/skeleton, `deleteOpen`, `onPaired→load`,
  `webhook-updated`, `sent`/`settled→messagesRefresh`, admin-gating e contrato
  CSS-gated do aside/slideover (sem `v-if`, sem fetch levantado) (task 4.1).
- `TestSendCard`/`MessagesCard`: emits `paired`/`updated`/`sent`/`settled`
  intactos; `data-testid`s estáveis (`pairing-card`, `webhook-card`,
  `test-send-card`, `messages-card` + novos `stat-*`, `channels-card`,
  `group-detail`, etc.) (task 4.2).
- Validações espelhando o backend (rune-count): grupo nome ≤25, status
  texto/caption 1..700 + kind image|video, perfil nome 1..100 / recado ≤500,
  privacy allowlists exatas; group-create 201 com `invite_code` vazio
  reconcilia via GET invite, nunca retenta create.
- Auth: nenhum header `apikey:` no manager (só cookie de sessão); token
  Chatwoot write-only (input separado, `type="password"`, limpo após save).
- `tasks.md`: itens 1.1–5.2 marcados `[x]`; **5.3 mantido `[ ]`** (matriz manual
  — ver §4).

## 4. NÃO EXECUTADO (sem backend vivo ou browser neste ambiente)

> **Waiver registrado 2026-09-17 (Opção 1):** o usuário aceitou arquivar com a
> matriz manual 5.3 registrada como risco abaixo, sem execução interativa.
> `tasks.md` 5.3 permanece `[ ]` por honestidade; o arquivamento prossegue com
> esse warning explícito.

- Matriz manual 5.3: conectado + desconectado × 7 abas (happy-path + display
  409/422) × mobile slideover + desktop × light/dark × admin vs user (aba de
  chave oculta) × Chatwoot habilitado vs desabilitado.
- Fluxos interativos: pair-phone Connect-first + expiração, revoke/mark-read/
  presence, grupos (foto/participantes/invite/join/leave), newsletters
  follow/unfollow, status publish/delete, profile/privacy save, Chatwoot
  config/import parcial/command, delete modal, toasts 501/`chatwoot_disabled`.

## 5. Code review (subagente revisor, 2026-09-17)

- Escopo confirmado: diff `58b60d9..ceb595b` = 20 arquivos, só `manager/`.
- Assessment: **Ready with notes** — 0 Critical, 1 Important, 5 Minor.
- Important (NÃO corrigido nesta sessão — ver decisão em §6):
  `InstanceHeaderStats.vue:20-25,117-122` + `[id].vue:37` — `chatwootState`
  inicia `false` ("até visitar a aba integrations"), então o stats strip
  renderiza Disabled antes de qualquer fetch; estado desconhecido (não
  carregado) é confundido com desabilitado real. Proposta: terceiro estado
  (`undefined` → `common.notSet`), mostrando Enabled/Disabled só após
  `chatwoot-loaded`.
- Minors (notas, sem mudança de código): (1) `ChannelsCard.vue:28-31`
  labels `statusKinds` hardcoded fora do `t()` — única brecha i18n do diff;
  (2) `ChatwootCard.vue:154` `void props.status` para silenciar lint —
  prop aceita mas não usada em gating; (3) `PrivacyCard.vue:19-23,56-61`
  refs iniciam `'all'` e `onSave` envia os 5 campos — save antes do load
  sobrescreveria settings reais, desabilitar save até `privacy !== null`;
  (4) `GroupDetail.vue:134-147` banners notice+failure empilhados após
  reconcile com erro — ruidoso mas aceitável; (5) `MessageComposer.vue:170`
  `selectable_count` sem teto client-side (422 verbatim cobre, mas clamp
  seria mais amigável).

## 6. Decisão de escopo

- O review mandaria corrigir o Important antes de prosseguir, mas
  "prosseguir" aqui = merge — que NÃO acontece nesta sessão (menu da skill
  `finishing-a-development-branch` será apresentado e a decisão é humana).
  `tasks.md` é o contrato de escopo e seus 15 itens estão cobertos pela
  verificação; nenhuma correção foi aplicada para não expandir o escopo sem
  ordem. O Important + minors seguem como follow-ups propostos (ver
  `retrospective.md` §5).
- **Adendo Opção 1 (2026-09-17):** trabalho já está em `main` (`ceb595b`);
  "merge para main" = commitar os artefatos openspec untracked + este
  `verify.md` atualizado. Arquivamento com 5.3 como risco aceito, por ordem
  explícita do usuário.

## 7. Commits do change (em `main`)

- `8a24727 feat(manager): extend instance detail API types`
- `520ddef feat(manager): add instance detail i18n copy`
- `ecad1d6`, `15dda6c`, `d79ac0c`, `4a19f22`, `a956835` (fixes de tipos/envelopes)
- `a12332f`, `11c50d9` (cards de grupos/profile/chatwoot)
- `8feaa4c`, `1e26fe7` (header stats + composer)
- `95ea454 feat(manager): rework instance detail into sectioned layout`
- `efee584 feat(manager): delegate detail cards without contract churn`
- `ceb595b fix(manager): satisfy lint for instance detail leaves`
- Pendente: commitar `openspec/changes/instance-detail-redesign/` (artefatos
  do change, hoje untracked) + `verify.md` + `retrospective.md` + `tasks.md`
  atualizado — NADA commitado nesta sessão sem ordem humana.
