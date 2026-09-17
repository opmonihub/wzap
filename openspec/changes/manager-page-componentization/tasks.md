# Tasks — manager page componentization

Scope contract: cada item é uma frase; verificação ao final de cada item.
Detalhe executável em `plan.md` por ID.

- [x] 1.1 Extrair `components/shared/PageState.vue` (skeleton + erro com retry) e usar em `pages/index.vue`, verificado com `pnpm --dir manager typecheck`.
- [ ] 1.2 Extrair `components/shared/DataTableToolbar.vue` (busca com debounce + slots + dropdown de colunas), verificado com `pnpm --dir manager lint`.
- [ ] 1.3 Extrair `components/shared/DataTableFooter.vue` (selectedOf + UPagination) e usar nas duas listas, verificado com `pnpm --dir manager typecheck`.
- [ ] 2.1 Criar `manager/app/utils/accountQuota.ts` com `parseQuota` movido de `pages/accounts/index.vue`, verificado com `pnpm --dir manager typecheck`.
- [ ] 2.2 Extrair `components/accounts/CreateAccountModal.vue` do modal inline de `pages/accounts/index.vue`, verificado com `pnpm --dir manager lint` + abrir/criar conta manualmente.
- [ ] 2.3 Extrair `components/accounts/EditQuotaModal.vue` do modal inline de quota, verificado com submit de quota manualmente + `pnpm --dir manager typecheck`.
- [ ] 2.4 Extrair `components/accounts/DeleteAccountModal.vue` do modal inline de delete, verificado com delete manual (incl. 409 owns-instances) + `pnpm --dir manager typecheck`.
- [ ] 2.5 Extrair `components/accounts/AccountsTable.vue` (toolbar + UTable + footer + seleção) e afinar `pages/accounts/index.vue`, verificado com matriz manual da tela accounts + `pnpm --dir manager lint`.
- [ ] 3.1 Extrair `components/instances/InstancesTable.vue` e afinar `pages/instances/index.vue` reutilizando os 3 modais existentes, verificado com matriz manual da tela instances + `pnpm --dir manager lint`.
- [ ] 4.1 Extrair `components/instances/detail/InstanceSectionNav.vue` do `[id].vue`, verificado com troca das 7 seções manualmente + `pnpm --dir manager typecheck`.
- [ ] 4.2 Extrair `components/instances/detail/InstanceOverviewSection.vue` (info + disconnect + rename + pairing), verificado com rename/disconnect manual + `pnpm --dir manager typecheck`.
- [ ] 4.3 Extrair `components/instances/detail/InstanceMessagesSection.vue` preservando o contrato aside/slideover, verificado em viewport lg e mobile + `pnpm --dir manager lint`.
- [ ] 4.4 Extrair `InstanceGroupsSection.vue` + `InstanceChannelsSection.vue` (wrappers finos), verificado com abertura das seções + `pnpm --dir manager typecheck`.
- [ ] 4.5 Extrair `InstanceProfileSection.vue` (Profile + Privacy + DeviceActions), verificado com abertura da seção + `pnpm --dir manager typecheck`.
- [ ] 4.6 Extrair `InstanceIntegrationsSection.vue` (Webhook + Chatwoot, re-emitindo `webhook-updated`), verificado com save de webhook manual + `pnpm --dir manager typecheck`.
- [ ] 4.7 Extrair `InstanceSettingsSection.vue` (chave admin + delete, emitindo `changed`/`delete-requested`), verificado com generate/revoke manual + `pnpm --dir manager typecheck`.
- [ ] 4.8 Afinar `pages/instances/[id].vue` compondo nav + seções + `DeleteInstanceModal`, verificado com matriz manual do detalhe + `pnpm --dir manager lint`.
- [ ] 5.1 Extrair `components/overview/OverviewHeader.vue` e afinar `pages/index.vue`, verificado com overview manual (fallback notice + retry) + `pnpm --dir manager lint`.
- [ ] 5.2 Extrair `components/auth/LoginForm.vue` e afinar `pages/login.vue`, verificado com login sucesso/falha manual + `pnpm --dir manager typecheck`.
- [ ] 6.1 Rodar gates finais (`typecheck`, `lint`, `build`, `gofmt -l .` vazio) e a matriz manual das 5 rotas × admin/user × light/dark, verificado com `pnpm --dir manager build` verde.
