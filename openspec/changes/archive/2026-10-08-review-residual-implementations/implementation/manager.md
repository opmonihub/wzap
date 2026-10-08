# Task 3 — Manager: relatório de implementação

Worktree: `/home/obsidian/dev/wzap/.worktrees/review-residual-implementations`.
Branch: `codex/review-residual-implementations`.
Escopo: tasks 3.1–3.5 da change `review-residual-implementations`.
Skills aplicadas: systematic-debugging, test-driven-development com writing-good-tests, verification-before-completion.

## Arquivos e causas corrigidas

| Task | Evidência no código final | Causa e correção |
| --- | --- | --- |
| 3.1 | `manager/app/components/instances/ProfileCard.vue:36` | Save sempre enviava name e status_text, fazendo o recado-only receber o 501 reservado a name. Agora constrói UpdateProfileInput somente com campos alterados, valida apenas campos presentes, conserva status_text vazio explícito e aplica defaults ao perfil carregado. Nome realmente alterado continua no pedido e preserva o aviso 501 atômico, sem toast de sucesso nem descarte da edição. Foto 501 continua com aviso. Save sem alteração não envia pedido. |
| 3.2 | `manager/app/pages/instances/[id].vue:61` e `:92` | load reutilizava route.params.id mesmo depois de rename, e updated só substituía instance. Agora associa a instância carregada à referência da rota e recarrega seu UUID; um alias invalidado por rename é substituído por URL de UUID, preservando section, demais query params e hash. Mudança de instância carrega a nova referência; respostas/updates da rota anterior são ignorados. Alias ainda válido não é reescrito sem necessidade. |
| 3.3 | `manager/app/components/instances/MessageComposer.vue:144` | Number de texto vazio/espaços produzia zero válido. Presença de ambos os textos é verificada antes da conversão; limites e finitude continuam validados, e zero explícito continua sendo enviado. |
| 3.4 | `manager/app/components/instances/ChannelsCard.vue:203`, `manager/i18n/locales/en.json:427` | O v-for filtrava a linha inteira pela presença de text/caption, escondendo mídia sem legenda e Delete. Toda entrada agora tem linha, tipo, ID e Delete; só a bolha de conteúdo depende de texto/legenda. Estado vazio permanece condicionado às coleções vazias. Copy nova: instances.channels.statusType. |
| 3.5 | `manager/app/components/instances/detail/InstanceSettingsSection.vue:82` e `:238` | Revoke exigia keySeen, que representa somente o cache deste navegador, e estava no ramo oculto por freshKey. Revoke agora fica disponível no card administrativo em ambos os estados. Confirmação permanece obrigatória; sucesso limpa cache/key exibida, falha conserva ambos. Gating administrativo é revalidado após a confirmação e revogação é bloqueada durante geração. |

Testes adicionados:

- `manager/app/components/instances/ProfileCard.test.ts` — 8 casos.
- `manager/app/pages/instances/detail.test.ts` — 5 casos.
- `manager/app/components/instances/MessageComposer.test.ts` — 16 casos.
- `manager/app/components/instances/ChannelsCard.test.ts` — 4 casos.
- `manager/app/components/instances/detail/InstanceSettingsSection.test.ts` — 8 casos.
- `manager/tests/componentHarness.ts` — compiler-sfc disponível através de Vue, transpile com TypeScript já instalado e renderer Vue em memória. SFCs e scripts reais são avaliados; imports e estado são novos em cada avaliação/montagem. Somente compilação por filename/conteúdo é cacheada.

Sem alterações Go, contrato REST, specs, AGENTS, package.json, lockfile, Nuxt modules ou dependências. Sem commit/merge/deploy. O coordenador mantém tasks e demais artefatos da change.

## Preparação e comandos

`pnpm --dir manager test ...` inicialmente parou antes do Vitest com ERR_PNPM_UNSAFE_MODULES_DIR: pnpm 11 tentou instalar automaticamente e recusou remover o node_modules que é symlink para as dependências existentes. Nenhuma instalação foi realizada. Os scripts foram então executados com a opção local de CLI `--config.verify-deps-before-run=false`, sem mudar configuração ou dependências.

O primeiro teste com essa opção detectou a ausência da `.nuxt/tsconfig.app.json` do worktree. Preparação executada com sucesso:

```sh
pnpm --config.verify-deps-before-run=false --dir manager exec nuxt prepare
```

## RED antes de GREEN

Comandos executados antes e depois da alteração correspondente:

```sh
pnpm --config.verify-deps-before-run=false --dir manager test app/components/instances/ProfileCard.test.ts
pnpm --config.verify-deps-before-run=false --dir manager test app/pages/instances/detail.test.ts
pnpm --config.verify-deps-before-run=false --dir manager test app/components/instances/MessageComposer.test.ts
pnpm --config.verify-deps-before-run=false --dir manager test app/components/instances/ChannelsCard.test.ts
pnpm --config.verify-deps-before-run=false --dir manager test app/components/instances/detail/InstanceSettingsSection.test.ts
```

| Task | RED observado | GREEN focado |
| --- | --- | --- |
| 3.1 | 6 failed / 2 passed. Recado Closed/vazio incluía name: Shop; nome carregado vazio bloqueava recado; Save inalterado enviava PATCH; recado inicial com 501 caracteres bloqueava nome-only; nome-only enviava status_text inalterado. Nome+recado e foto já mantinham 501. | 8 passed / 0 failed. |
| 3.2 | 5 failed. URL permaneceu OldName; refresh chamou OldName; navegação a OtherName manteve o UUID antigo; lookup atrasado e evento updated da seção anterior sobrescreveram a nova instância. | 5 passed / 0 failed. |
| 3.3 | 6 failed / 10 passed. Vazio, espaços, tab e um campo ausente geraram POST e polling; cada caso esperava zero requests. Zeros explícitos, extremos válidos e limites inválidos já estavam corretos. | 16 passed / 0 failed. |
| 3.4 | 3 failed / 1 passed. Imagem/vídeo sem legenda produziram zero linhas; status textual não tinha ID visível. Coleção vazia já renderizava UEmpty. | 4 passed / 0 failed. |
| 3.5 | 6 failed / 2 passed. Quatro casos não encontraram Revoke sem cache, com storage indisponível ou freshKey; retirada de acesso admin enquanto o confirm estava aberto ainda executou DELETE; geração em curso não bloqueava Revoke. Cancelamento e gating visual do usuário comum já estavam corretos. | 8 passed / 0 failed. |

As falhas de configuração/harness encontradas antes das reproduções úteis foram corrigidas antes de contabilizar RED: resolução ESM/default de SFCs importados e normalização de model-value nos primitives. Não foram usadas como prova de bug de produção.

## Fluxos exercitados

- Perfil: SFC real e useInstanceProfile real, com boundary API que aplica recado e responde 501 atomicamente se name estiver presente. Os testes editam inputs renderizados e clicam Save/upload; verificam payload, estado retornado, aviso e ausência de sucesso no 501.
- Rename: página real, PageState real e useInstances real. O evento updated representa o PATCH bem-sucedido; o fixture remove o alias antigo. Testes exercitam router.replace, paired/changed posteriores, remontagem da URL canonicalizada e navegação a outra instância. Corridas usam promises controladas, sem sleeps.
- Localização: composer e composables reais; v-model de recipient, tab e coordenadas é executado. Timer fake avança apenas o polling de entrega já existente. Testes verificam zero requests para inválidos e payload/eventos sent/settled para válidos.
- Status: ChannelsCard e useInstanceChannels reais; imagem/vídeo sem legenda permanecem identificáveis, Delete chama o endpoint correto e refresh torna a coleção vazia. Texto e caption conservam seu conteúdo e ações independentes.
- Revogação: seção, OneTimeKeyDisplay, useInstances, helpers reais de localStorage e useConfirmDelete reais. Overlay fornece uma promise de decisão; nenhuma API é chamada antes da confirmação. Exercitados storage que lança, cancelamento, sucesso, erro, segredo recém-gerado, usuário comum, autorização alterada durante confirmação e geração pendente.

Primitives do Nuxt UI e conteúdo das subseções não pertinentes são substituídos por elementos no renderer; handlers/template dos SFCs alvos e composables acima não são copiados nem simulados. Não houve teste de navegador/DOM, serviço Go ativo ou chamadas reais WhatsApp/Chatwoot. A validação é de fluxo de componentes e dos payloads emitidos, além dos gates de compilação.

## Gates completos e resultados finais

```sh
pnpm --config.verify-deps-before-run=false --dir manager test
node --test manager/tests/instanceName.test.mjs
pnpm --config.verify-deps-before-run=false --dir manager lint
pnpm --config.verify-deps-before-run=false --dir manager typecheck
pnpm --config.verify-deps-before-run=false --dir manager build
git diff --check -- manager
```

| Gate | Resultado |
| --- | --- |
| Vitest completo | Exit 0; 8 files / 48 tests passed, 0 failed. 41 regressões novas + 7 testes existentes. |
| Node instanceName | Exit 0; 7 passed, 0 failed/skipped/cancelled. |
| ESLint | Exit 0; 0 errors. Dois warnings preexistentes vue/no-required-prop-with-default: DataTableToolbar.vue:9 e PageState.vue:6. |
| Nuxt typecheck | Exit 0. |
| Nuxt generate | Exit 0; 7 rotas geradas; manager/.output/public/index.html existe. Avisos observados: PLUGIN_TIMINGS, imports H3 não usados dentro do Nitro e mensagem de HTML não prerenderizado com ssr: false. |
| Diff check | Exit 0 para manager. package.json, pnpm-lock.yaml, nuxt.config.ts e vitest.config.ts sem diff. |

O primeiro gate completo teve 47 passed / 1 timeout no teste ChannelsCard de imagem sem legenda, em 29.43s; o caso havia passado focado. Compilar/transpilar todo SFC em cada teste consumia CPU sob execução concorrente. Depois do cache exclusivo de compilação por path/conteúdo, a mesma suite terminou 48/48 em 15.19s; tempo agregado dos testes caiu de 37.72s para 13.28s. Nenhum timeout nem número de workers foi alterado. Estado, imports e montagens continuam isolados por teste. A saída Vitest contém apenas a linha informativa do Vue sobre Suspense experimental.

Build gerou artefatos ignorados e removeu `.output/public/.gitkeep` criado pelo coordenador; a restauração/exportação do placeholder foi comunicada e pertence ao coordenador. Nuxt também utiliza automaticamente `node_modules/.cache/nuxt` pelo symlink de dependências. As fontes da árvore principal não foram editadas.
