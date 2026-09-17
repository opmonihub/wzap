# Brainstorm: manager-dashboard-frontend

Raw capture of the brainstorming dialogue. Decisions live here; design.md holds the refined technical choices.

## Contexto levantado
- `manager/app/pages`: index overview estático (4 UCard), instances lista já canônica, [id] detalhe coluna única, accounts tabela, login UAuthForm.
- Template `dev/research/dashboard`: Home com navbar sino+criar, toolbar período, HomeStats/HomeChart/HomeSales; customers com filtros avulsos e UTable com ui bordas; settings centralizado; inbox dois painéis + slideover mobile.
- `useDashboard` local usa `g-h/g-i/g-a/g-s/n`; template usa `g-h/g-i/g-c/g-s/n`.
- Deps gráfico ausentes no manager: `@unovis/vue`, `date-fns`.
- Backend: sem endpoint de stats; quotas contam via `CountByOwner/CountAll`; listagem com filtro por owner por escopo.
- OpenSpec existente `manager-dashboard-refactor` com skip_specs:true; decidimos change nova.

## Decisões do usuário
1. Escopo: todas as páginas juntas.
2. Números do overview: contagem local da listagem atual (depois liberou endpoint de métricas aditivo).
3. Gráfico: sim, Unovis + date-fns.
4. Abordagem: "usar os mesmos componentes base" (interpretado como Home fiel com stats+gráfico, só primitivas canônicas).
5. Backend: permite endpoint de métricas; resto de internal/ congelado.
6. Change: criar change nova `manager-dashboard-frontend`.

## Trade-offs registrados
- Endpoint novo aditivo vs contagem local pura: endpoint dá total correto sem acumular cursor; fallback local mantém funcionamento se falhar.
- Gráfico de created_at vs série temporal real: série temporal exigiria evento novo; created_at atende período sem backend novo.
- Fork total descartado: traria inbox/teams/mocks inúteis.
- Skin mínima descartada: menos valor visual.
