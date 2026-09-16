## 1. Backend stats aditivo (F1)

- [x] 1.1 Adicionar `GET /instances/stats` com escopo e testes de handler em `internal/httpapi/` e verificar `go test ./internal/httpapi/ -run TestInstanceStats -count=1`.
- [x] 1.2 Regenerar swagger docs e verificar `go test ./internal/httpapi/ -run TestSwagger -count=1`.

## 2. Overview Home (F2)

- [x] 2.1 Adicionar `@unovis/vue` + `date-fns` via `pnpm --dir manager install` e verificar lockfile atualizado.
- [x] 2.2 Criar `composables/useOverview.ts` com stats + fallback local e verificar `pnpm --dir manager typecheck`.
- [x] 2.3 Criar `components/overview/*` (Stats, Chart client+server, Recent) e reescrever `pages/index.vue` como Home e verificar `pnpm --dir manager build`.
- [x] 2.4 Adicionar chaves `overview.stats.*` e `overview.chart.*` em `i18n/locales/en.json` e verificar `pnpm --dir manager lint`.

## 3. Listas e detalhe no padrão (F3)

- [x] 3.1 Adotar `ui` de tabela do template + footer de selecionados em `pages/instances/index.vue` e `pages/accounts/index.vue` e verificar `pnpm --dir manager build`.
- [x] 3.2 Reorganizar `pages/instances/[id].vue` no split settings+inbox sem mudar cards e verificar em viewport desktop e mobile.

## 4. Verificação final (F4)

- [x] 4.1 Rodar verificação completa (`pnpm lint`, `typecheck`, `nuxt generate`, `go test ./manager/... ./internal/httpapi/... -count=1`, `go vet ./...`, `gofmt -l .`) e verificar tudo verde.
