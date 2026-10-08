# QA geral do frontend — wzap manager

Período: 2026-10-07 a 2026-10-08. Navegador: Chrome 154 com agent-browser 0.38.1 na primeira parte e 0.38.2 após a atualização solicitada, sessão própria. Interface: build atual do Nuxt em http://127.0.0.1:3101/manager/.

As **10 falhas confirmadas foram corrigidas**, revisadas por subagentes e aplicadas ao workspace principal. As reproduções abaixo registram o estado anterior. QA encerrado, sem registros temporários ativos.

## Ambiente e limites

A interface atual é servida por um proxy temporário local. O gateway existente em :8081 ainda responde coleções no contrato antigo (`data.items` com wrappers singulares). Esse ambiente produziu um erro de renderização ao abrir Overview antes da adaptação. O proxy passou a converter apenas essas coleções para o contrato atual (`data.instances`, `data.users`, etc.). Isso é uma adaptação do ambiente de QA, não uma correção no produto; os testes não validam a combinação com o backend recém-refatorado em execução.

Login por conta de administrador descartável `@qa.invalid`, pois as credenciais de seed configuradas não foram aceitas. CRUD limitado a registros de QA. Credenciais e estado de sessão ficam fora do repositório; evidências ocultam segredos. Ações reais de WhatsApp estão bloqueadas. Casos de sessão conectada foram exercitados com fixtures locais; uploads de arquivos foram locais, sem entrega real ao WhatsApp.

## Verificações automatizadas

- Vitest final: **117/117 testes, 14 arquivos**; início da QA tinha 48/48.
- Node: 7/7 testes de nomes de instâncias.
- Typecheck: passou.
- ESLint: 0 erros, 2 avisos preexistentes (`DataTableToolbar.vue`, `PageState.vue`).
- Build Nuxt de produção: passou; 122 hashes de fontes coincidem com o workspace principal e a build da etapa visual final.
- Go build ./...: passou com os assets atualizados.
- OpenSpec validate strict: passou.
- Revisões independentes: PASS, sem bloqueios P1/P2.

## Problemas confirmados e corrigidos

### ISSUE-001 — Validação do login exibe mensagens internas (P2 / média)

Categoria: UX / conteúdo. URL: `/manager/login`.

Ao enviar o formulário recém-aberto sem preencher os campos, ambos exibem `Invalid input: expected string, received undefined`. A validação impede o envio corretamente, mas deve orientar o usuário com mensagens como “Enter your email” e “Enter your password”. Reproduzido três vezes; nenhuma requisição de login foi enviada nesses casos.

1. Abrir o login sem preencher os campos. [Captura](screenshots/issue-001-retry-step-1.png).
2. Clicar em **Sign in**. [Resultado anotado](screenshots/issue-001-retry-result.png).

[Vídeo da reprodução](videos/issue-001-login-errors-retry.webm).

### ISSUE-002 — Contraste insuficiente no botão e erros do login (P1 / alta)

Categoria: acessibilidade. URL: `/manager/login`.

Axe 4.12.1 confirmou falha em WCAG 1.4.3 AA: texto branco do botão **Sign in** sobre verde `#00c16a` tem contraste **2,37:1**; mensagens vermelhas `#fb2c36` sobre branco têm **3,8:1**. Textos de 14 px precisam de pelo menos **4,5:1**. Conferência visual e estilos computados confirmam as cores; marcas da logo não foram contabilizadas.

[Evidência visual](screenshots/issue-001-retry-result.png) · [Resultado axe](login-a11y.json). Problema estático, vídeo dispensável.

### ISSUE-003 — Botão de voltar sem nome acessível (P2 / média)

Categoria: acessibilidade. URL: `/manager/instances/{id}` (todas as seções).

A seta de voltar no cabeçalho não tem texto, `aria-label`, `aria-labelledby` nem `title`. A árvore de acessibilidade identifica apenas “button”; axe confirma `button-name`. Um leitor de tela não consegue identificar a ação. Nomear o controle, por exemplo “Back to instances”.

[Captura do cabeçalho](screenshots/06-instance-detail-disconnected.png) · [Resultado axe](detail-a11y.json).

No estado anterior, o problema de contraste da ISSUE-002 também se repetia no detalhe da instância: aba ativa Overview (2,37:1), botões Retry/Get pairing code (2,37:1) e aviso informativo azul (3,19:1). O texto do aviso vermelho produzido pelo bloqueio de QA usa os mesmos tokens e tem 3,15:1, mas seu conteúdo pertence ao ambiente de teste.

### ISSUE-004 — Cota numérica impede criar conta e editar limites (P1 / alta)

Categoria: funcional. URL: `/manager/accounts`, modal **New account**.

Com email e senha válidos, preencher **Instance quota** com `0` ou `2` impede o envio. O campo é um input numérico, mas a validação rejeita seu valor como `Invalid input: expected string, received number`. Não chega nenhuma requisição `POST /users` ao gateway. Limpar apenas esse campo permite criar a mesma conta; os demais dados permanecem iguais. Reproduzido com `0` duas vezes e `2` uma vez. A interface deve aceitar inteiros >= 0, preservando 0 como ilimitado.

1. Preencher os campos válidos e cota `0`. [Estado inicial](screenshots/issue-004-step-1.png).
2. Alterar a cota para `2` e clicar **Create account**. [Erro anotado](screenshots/issue-004-result.png).
3. Limpar a cota e reenviar: a conta é criada.

[Vídeo](videos/issue-004-account-quota.webm). Edição também falhou ao alterar a cota para `3`, com a mesma mensagem e nenhuma requisição PATCH. [Captura da edição](screenshots/issue-004-edit-result.png) · [Vídeo da edição](videos/issue-004-edit-quota.webm).

### ISSUE-005 — Rótulos dos eventos do webhook alteram o primeiro evento (P1 / alta)

Categoria: funcional / acessibilidade. URL: `/manager/instances/{id}?section=integrations`.

Os dez checkboxes de eventos e os respectivos labels compartilham o mesmo ID gerado (`v-0-0-28` nesta sessão). Ao clicar no texto **receipt**, o checkbox **message** é marcado; receipt continua desmarcado. Repetido após “None” com **group.info**, com o mesmo resultado. A árvore de acessibilidade também chama todos os checkboxes de “message”. Isso permite configurar assinaturas diferentes das pretendidas. Clicar diretamente no quadrado do checkbox é uma alternativa; atribuir um ID exclusivo por evento corrige a associação.

1. Clicar **None** para desmarcar todos. [Captura](screenshots/issue-005-step-1.png).
2. Clicar no texto **receipt**. [Resultado anotado](screenshots/issue-005-result.png).

[Vídeo](videos/issue-005-webhook-labels.webm). Na reprodução anterior não houve gravação; no reteste, somente a instância descartável de QA foi configurada.

### ISSUE-006 — Seletores de arquivos sem nome acessível (P2 / média)

Categoria: acessibilidade. URLs: seções Profile, Groups e Channels da instância.

O seletor de arquivo aparece como um botão sem nome na árvore acessível; o texto “Photo” existente em Profile não está associado ao controle. Axe confirma `aria-command-name` nos seletores. Nomear o alvo clicável e associar o label para permitir selecionar arquivos com leitores de tela. O problema também aparece no formulário de mídia de Messages na inspeção da árvore.

[Profile](screenshots/17-profile-recado-saved-fixture.png) · [Groups](screenshots/19-groups-desktop-fixture.png) · [Resultado axe](profile-a11y.json).

### ISSUE-007 — Abas do detalhe ficam ilegíveis no mobile (P2 / média)

Categoria: responsividade / UX. URL: `/manager/instances/{id}`, largura 390 px.

As sete abas são comprimidas na mesma linha. Overview recebe 6 px para um rótulo de 61 px; Messages 9/66 px; Groups e Profile ficam com largura de rótulo 0; Settings 3/55 px. Assim aparecem fragmentos de texto e ícones, dificultando identificar a seção. O documento não vaza horizontalmente; a falha é a compressão do menu. A árvore de acessibilidade mantém os nomes completos. Usar um menu mobile com nomes legíveis, rolagem horizontal das abas ou uma seleção de seção.

[Captura anotada em Settings](screenshots/25-settings-mobile-top-dark.png) · [Messages](screenshots/26-messages-mobile-dark.png) · [Integrations em tema claro](screenshots/22-integrations-mobile-light.png).

### ISSUE-008 — Menu Display vazio na tabela de instâncias (P2 / média)

Categoria: UX / estado de controle. URL: `/manager/instances`, modo Table.

Clicar em **Display** abre um menu com um grupo vazio e nenhum item interativo. A tabela atual possui apenas colunas obrigatórias (`enableHiding: false`), portanto não há opções de exibição. O toolbar compartilhado ainda oferece o dropdown. Ocultar o controle quando a lista de colunas opcionais estiver vazia, mantendo o menu de Accounts disponível.

[Captura do menu vazio](screenshots/30-instance-display-empty-menu.png). DOM e árvore acessível confirmam `menu "Display" → group` sem conteúdo. Problema estático, vídeo dispensável.

### ISSUE-009 — Linhas de tabela viram botões com controles internos (P2 / média)

Categoria: acessibilidade. URL: /manager/instances, modo Table.

O onSelect do UTable atribui role=button e tabindex=0 às linhas, que já contêm links, checkbox e botões. Axe confirmou nested-interactive nas duas linhas. A correção mantém a semântica nativa e delega cliques ao modelo de linhas exibidas, preservando filtros/ordenação. Links e checkboxes continuam sendo alvos do teclado.

[Antes](screenshots/issue-009-before.png) · [Axe antes](issue-009-before-a11y.json) · [Depois](screenshots/after-instances-table-dark.png) · [Axe depois](after-instances-table-dark-a11y.json).

### ISSUE-010 — Miniaturas selecionadas sem texto alternativo (P2 / média)

Categoria: acessibilidade. URLs: Profile, Groups, Channels e ambos os uploads de Messages.

Ao selecionar uma imagem, o FileUpload renderiza img sem alt. Axe confirmou image-alt. Um preview compartilhado agora usa o nome do arquivo como alt, conserva a miniatura/remoção e revoga URLs locais ao trocar/remover ou desmontar.

[Antes](screenshots/issue-010-before.png) · [Axe antes](issue-010-before-a11y.json) · [Depois](screenshots/after-profile-file-picker.png) · [Axe depois](after-profile-file-picker-a11y.json).

## Reteste final

| Falha | Resultado e evidência |
|---|---|
| 001 | Login intocado mostra Enter your email / Enter your password, sem POST. [Depois](screenshots/after-login-light.png). |
| 002 | Contraste normal/hover corrigido nos dois temas; também corrigido avatar muted que media 4,39:1. [Medições](after-rendered-button-contrast.json), [light](after-message-error-light-hover-a11y.json), [dark](after-message-error-dark-hover-a11y.json), [grupo](after-group-controls-a11y.json). |
| 003 | Back to instances tem nome acessível e navega por clique. [Depois](screenshots/after-webhook-labels-persist.png). |
| 004 | Contas reais de QA criadas com 0 (Unlimited) e 2, editada 2→3. [Zero](screenshots/after-account-quota-zero.png), [edição](screenshots/after-account-quota-edit.png). |
| 005 | receipt/group.info selecionam os eventos corretos; save/reload real conserva ambos, message falso e dez IDs únicos. [Prova](after-webhook-request-proof.json). |
| 006 | Photo/File, Participants action, Kind e Default disappearing timer têm nomes corretos; upload, seleção/remoção funcionam. [Groups](screenshots/after-group-controls.png), [Channels](screenshots/after-channel-file-picker.png). |
| 007 | Sete seções selecionadas em 320/390px e nos dois temas, sem overflow; controle 44px com rótulos completos. [390 light](after-mobile-navigation-390-proof.json), [320 light](after-mobile-navigation-light-320-proof.json), [320 dark](after-mobile-navigation-dark-320-proof.json), [390 dark](after-mobile-navigation-dark-390-proof.json). |
| 008 | Instâncias sem Display vazio; Accounts permite ocultar/restaurar Role. [Tabela](screenshots/after-instances-table-light.png). |
| 009 | Axe sem nested-interactive; Space seleciona checkbox sem navegar; menu Edit/cancel independente; clique após ordenar/filtrar e Enter no link abrem UUID correto. [Ordem](after-table-sorted-row-proof.json), [filtro](after-table-filtered-row-proof.json), [teclado](screenshots/after-instances-table-keyboard.png). |
| 010 | Miniatura com alt=qa-pixel.png carrega nos cinco pontos; remoção e axe passam. [Profile](after-profile-file-picker-a11y.json), [Groups](after-group-controls-a11y.json), [Channels](after-channel-file-picker-a11y.json), [Test send](after-test-send-file-picker-a11y.json), [Composer](after-composer-file-picker-a11y.json). PDF mantém detecção document e ícone: [prova](after-document-preview-proof.json). |

Botões renderizados: primary light 7,19:1 normal / 9,15:1 hover; primary dark hover 6,06:1; Retry light hover 5,27:1 / dark 4,65:1; Delete dark 6,13:1 normal / 5,20:1 hover e light hover 5,28:1.

## Cobertura manual

- Login válido por cookie e erro de credenciais inválidas: exercitados.
- Validação de nome vazio e nome com espaço: bloqueia o envio.
- Criar instância sem referência externa: passou; chave mostrada uma vez e ocultada nas evidências.
- Excluir instância: exige digitar o nome exato; exclusão confirmada e chave removida.
- Renomear instância: passou, URL permanece por UUID e nome no cabeçalho atualiza.
- Revogar chave como admin sem chave local em cache: passou; cancelamento mantém a chave, confirmação retorna sucesso.

- Accounts: pesquisa/role, criação, edição de quota e Display exercitados.
- Instâncias: Card/Table, busca vazia/Clear filters, ordenação, seleção por Space, link por Enter e edição/cancelamento. Duas linhas na amostra: segunda página coberta pelo resolver/modelo, sem navegação real entre páginas.
- Ctrl+K/search de páginas e API docs: exercitados na QA inicial.
- Sete seções abertas no desktop e mobile; visual em 320/390/1440px, light/dark.
- Webhook: labels, None/All, save/reload real na instância descartável, webhook desabilitado.
- Profile (fixture): recado salvo somente com status_text; nome/foto 501 na QA inicial. Upload, remoção/reseleção confirmados.
- Groups/Channels (fixtures): lookup, convite, seletores; follower count 0/status sem caption preservados; exclusão do status exercitada.
- Messages (fixtures): 0,0 com 202; erro 500 controlado, Retry e recuperação; dois uploads e PDF. [Requisição](after-zero-location-request-proof.json).
- Settings: Off para duration "0", mudança 24h→Off, key/revoke e exclusão/cancelamento.
- Modal/menu: foco, Escape, teclado; logout final pela UI passou.
- agent-browser errors: nenhum erro JavaScript não tratado na leitura final.

## Acessibilidade: limites dos scans

Scans filtrados para WCAG 2 A/AA. Estados finais fechados citados nas evidências têm zero violações desses checks; não representam certificação completa. Incomplete inclui logo e previews que exigem inspeção humana.

USelect aberto produz avisos aria-hidden-focus no fundo e scrollable-region-focusable quando há scroll. Reka SelectContentImpl usa hide-others, FocusScope e bloqueia Tab; no browser, Tab permaneceu na opção, End alcançou Settings, Enter navegou e Escape restaurou foco. Opções são operáveis pelo teclado. [Scan aberto preservado](after-mobile-sections-light-390-keyboard-a11y.json). Os avisos estáticos do popup foram distinguidos das falhas de uso confirmadas.

## Limpeza e entrega

Removidos 1 instância e 4 usuários descartáveis; os dois UUIDs de instâncias de QA (incluindo a primeira já excluída) retornam 404. A instância e o usuário existentes permanecem com identidade preservada. [Prova](cleanup-proof.json).

Browser próprio/proxy encerrados; porta 3101 fechada. Credenciais/estado privados temporários removidos. Aplicados 27 deltas após comparação com o snapshot, preservando trabalho anterior; bundle gerado atualizado. Worktree retido, sem commit/deploy.

Logs: [Vitest](quality-tests.log), [nomes](quality-names.log), [lint](quality-lint.log), [typecheck](quality-typecheck.log), [build](quality-build.log). [Hashes verificados](verified-source-hashes.json).


## Ferramenta de navegador

Executados `npm install -g agent-browser` e `agent-browser install --with-deps`, ambos com sucesso. CLI atual 0.38.2; Chrome for Testing 155.0.8059.39 e dependências instalados. O Chrome baixado não inicia com a sandbox neste Ubuntu/AppArmor. A configuração do usuário aponta para `/usr/bin/google-chrome`, que inicia normalmente com sandbox. Atalho antigo em `~/.local/bin` alinhado com a instalação npm. Uma sessão própria de instalação abriu e fechou `about:blank` com os defaults; `doctor` com `AGENT_BROWSER_EXECUTABLE_PATH=/usr/bin/google-chrome` marcou 12 passes, 0 falhas e 1 aviso referente a outra sessão antiga, que foi preservada.
