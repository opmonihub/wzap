## Context

Console Nuxt 4 + `@nuxt/ui` 4.11 em `manager/app` (5 páginas, `ssr:false`, `baseURL /manager/`, embed Go via `nuxt generate`). Listas já usam `UTable` + `getPaginationRowModel()` direto. Template `nuxt-ui-templates/dashboard` em `/home/obsidian/dev/research/dashboard`. Backend sem endpoint de stats; contagens de quota usam `CountByOwner/CountAll`.

## Goals / Non-Goals

**Goals:**

- Overview vira Home do template com dados reais (`GET /instances/stats` + listagem).
- Listas/detalhe adotam o visual customers/settings/inbox sem mudar fluxos.
- Endpoint aditivo com escopo, sem migração.

**Non-Goals:**

- Sem mudar NATS, schema, envelopes, auth, quotas, webhook, chatwoot, envio; sem série temporal no backend; sem `GET /users/stats`.

## Decisions

- **`GET /instances/stats` com filtro por escopo no handler**: admin/global somam tudo, user soma só as próprias via `filterInstancesByOwner`-like sobre `List` acumulada ou query com owner. Escolhido acumular via `List` existente no service para não tocar no repository nesta change; alternativa query SQL nova descartada (exigiria método novo no repo + Postgres; faremos se o volume provar lento).
- **Fallback local no frontend**: se stats falhar, `useOverview` deriva da listagem; alternativa falhar a tela descartada (overview deve degradar).
- **Gráfico de `created_at` com `date-fns`**: `eachDayOfInterval`/`eachWeekOfInterval`/`eachMonthOfInterval` sobre itens carregados; alternativa endpoint de série descartada (fora do escopo).
- **Stats como `UPageCard` clicáveis para `/instances`**: igual ao template que liga para `/customers`; alternativa cards estáticos descartada.
- **Detalhe split sem mover lógica**: só classes e wrappers; cada card filho inalterado; alternativa reescrever cards descartada.

## Risks / Trade-offs

- [Risk] Acumular todas as páginas via cursor para stats locais pode ser lento com milhares de instâncias -> Mitigation: endpoint de stats é o caminho feliz; fallback acumula com limite de páginas e avisa.
- [Risk] Unovis aumenta o bundle do console -> Mitigation: `OverviewChart.client.vue` só carrega no client; versão server rende skeleton.
- [Risk] Nova rota aditiva precisa de swagger + teste de escopo -> Mitigation: anotação swag + testes de handler para admin/user/instance-key.
- [Risk] Rebuild do console invalida o embed Go -> Mitigation: `pnpm --dir manager build` antes de `go test ./manager/...`; CI já segue essa ordem.

## Migration Plan

Fases na mesma branch: F1 backend stats (handler+testes+swagger) → F2 overview (composable+componentes+i18n) → F3 listas/detalhe (só visual) → F4 verificação completa. Rollback por revert de commit de fase; nenhum passo exige migração de dados.
