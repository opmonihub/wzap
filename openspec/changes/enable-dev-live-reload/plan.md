# enable-dev-live-reload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** apply iniciado em worktree isolado; requisitos e escopo originais preservados.

**Goal:** Recarga automática de Go e Nuxt pela entrada pública existente, com encerramento seguro e dados preservados.

**Tech Stack:** Go 1.26, biblioteca padrão HTTP, Nuxt 4, Node 24, pnpm do manifest, Air v1.61.7 e Docker Compose.

**Spec:** proposal.md, design.md e specs/{wzap-http-routing,wzap-manager,wzap-operations}/spec.md nesta change.

## Global Constraints

- Go permanece em `127.0.0.1:8081`, API na raiz e manager em `/manager/`.
- Não adicionar dependências de aplicação, migrações, serviços de ferramentas ou alterações de CI.
- Air `v1.61.7`, debounce de 200 ms, `send_interrupt = true`, `full_bin = "exec ./tmp/wzap serve"`, `kill_delay = "12s"` e `stop_grace_period: 20s`.
- Não iniciar uma segunda réplica no mesmo banco nem apagar volumes ou dados.
- Não manter protocolo de arquivos PID nem procurar processos pelo nome; proteção mínima somente após falha da prova nativa.
- Nenhuma mudança no contrato REST ou nos eventos; produção continua com assets embutidos.

## Review Focus

- URLs inválidas com credenciais: erros identificam somente a variável; tarefa 1 cobre valor sensível.
- Paths escapados, Host e headers de encaminhamento: tarefa 2 cobre a requisição recebida por upstream real.
- WebSocket através dos middlewares: tarefa 3 cobre upgrade e comunicação bidirecional real.
- Sinais sucessivos e build concorrente: tarefa 5 exige filho artificial de 8 segundos e registro de início/fim/sinais.
- Overrides locais e projeto legado: tarefas 6 e 7 comparam containers/volumes e mantêm infraestrutura sem recriação.


**Contrato:** `tasks.md` define o escopo; `proposal.md`, `design.md` e as três delta specs definem os requisitos e decisões. Usar os IDs abaixo para registrar RED/GREEN e evidências durante apply; nenhum checkbox de implementação deve ser marcado durante a proposta.

**Arquitetura:** Go permanece em `127.0.0.1:8081`, API na raiz e manager em `/manager/`; Air e Nuxt dev usam fontes montados; produção continua com assets embutidos; workers permanecem no Go.

## Execution order

Tasks 1, 2, 3, then Task 5 (Air safety), then Tasks 4 and 6 as one integration deliverable (Nuxt + Compose), then Task 7. Original OpenSpec task IDs and acceptance criteria remain unchanged.

## Preparação para apply

- Criar ou reutilizar `.worktrees/enable-dev-live-reload` na branch `codex/enable-dev-live-reload`, conforme AGENTS; levar esta change para o worktree e preservar todas as alterações preexistentes no checkout principal.
- Usar Go 1.26, Node 24, o pnpm fixado no manifest e Air `v1.61.7`; não adicionar dependências de aplicação, migrações, serviços de ferramentas ou alterações de CI.
- Registrar baseline com `go test ./manager ./internal/config ./internal/httpapi/... -count=1` e `pnpm --dir manager test`; preservar o placeholder de embed no clone sem bundle e separar falhas ambientais de regressões.
- Fixar primeiro o contrato Go do proxy e então ajustar Nuxt; um escritor por arquivo, com implementação por lado conforme o workflow do projeto.
- Inspecionar projeto/container Go ativo, infraestrutura, mapeamentos locais e nomes dos volumes antes de qualquer execução de `serve`; os testes de Air usam apenas um filho artificial em diretório temporário, sem banco.

## 1. Configuração e proxy — tarefas 1.1–1.3

### Task 1: Configuração opcional (1.1)

**Arquivos:** `internal/config/config.go`, `internal/config/config_test.go`.

- [ ] Adicionar testes para variável vazia, endereços HTTP/HTTPS válidos e inválidos, incluindo credenciais, query, fragmento e prefixo de path; incluir a variável na limpeza de ambiente dos testes e exigir erro que não reproduza valores sensíveis.
- [ ] Confirmar RED, adicionar `Config.ManagerDevURL string` com padrão vazio e validação da URL conforme design, então rodar `go test ./internal/config -count=1` até GREEN.

### Task 2: Encaminhamento HTTP (1.2)

**Arquivos:** novo `manager/devproxy.go` e `manager/devproxy_test.go`; composição em `internal/httpapi/server.go` e cobertura em `internal/httpapi/manager_test.go`.

- [ ] Escrever testes com upstream `httptest.Server` e entrada HTTP real para `/manager?next=instances`, `/manager/`, página interna, asset com query, path escapado e Host público; confirmar falha por ausência do encaminhamento.
- [ ] Incluir upstream indisponível mesmo quando existir bundle estático utilizável, recuperação do upstream e requisições à API/probes/Swagger que nunca o alcançam; preservar cobertura atual do modo estático.
- [ ] Implementar o handler dev com `httputil.ReverseProxy`, `Rewrite` e `ErrorHandler` genérico `503`, mantendo o prefixo completo e a query; na composição, construir o handler uma vez e selecionar pelo campo de configuração.
- [ ] Rodar `go test ./manager ./internal/config ./internal/httpapi/... -count=1`; não alterar os formatos REST ou o atendimento estático sem configuração dev.

### Task 3: Upgrade WebSocket (1.3)

**Arquivos:** testes do proxy e de composição HTTP acima.

- [ ] Escrever teste de upgrade com servidor HTTP real, cliente de socket e frames WebSocket mínimos, usando somente biblioteca padrão; demonstrar RED para upgrade bloqueado ou fluxo interrompido.
- [ ] Verificar `101`, path/query/Host no upstream e mensagens nos dois sentidos através da composição HTTP com seus middlewares, não apenas um recorder em memória.
- [ ] Corrigir somente o necessário para preservar a capacidade de upgrade e rodar `go test ./manager ./internal/httpapi/... -count=1` até GREEN.

**Ponto de commit futuro:** configuração/proxy e testes, com prefixo `feat(api):`; Swagger só deve ser regenerado se annotations forem alteradas.

## 2. Nuxt — tarefa 2.1

### Task 4: HMR pela entrada pública (2.1)

**Arquivo:** `manager/nuxt.config.ts`.

- [ ] Preservar `ssr: false`, base `/manager/`, API na mesma origem e proxy Vite standalone; aplicar `NUXT_DEV_HMR_CLIENT_PORT=8081` somente ao cliente WebSocket de desenvolvimento conforme design.
- [ ] Com o Compose da tarefa 4.1 ativo, observar `101` pela origem `8081` e registrar os caminhos de assets/HMR, sem prefixo duplicado nem fallback para porta interna.
- [ ] Fazer alterações temporárias de texto em componente Vue e de CSS, confirmar o efeito automático no navegador e restaurar os fontes; não adicionar UI de diagnóstico ao produto.

**Ponto de commit futuro:** configuração HMR mínima, com prefixo `feat(manager):`.

## 3. Air — tarefas 3.1–3.3

### Task 5: Air nativo e proteção condicionada (3.1–3.3)

**Arquivos:** `.air.toml`, `Dockerfile.dev` quando necessário e novo teste `docker/dev_reload_test.go`.

#### 3.1 Configuração nativa e recuperação

- [ ] Criar harness Go que use Air real em diretório temporário e filho artificial, com registro de início/fim e conteúdo observado; não lançar a aplicação real nem escrever em seu `tmp` ativo.
- [ ] Aplicar o debounce e a configuração nativa definidos no design, observar Go/mod/sum/SQL e ignorar testes, caches e arquivos frontend; confirmar build automático após alteração e ausência de build Go por edição Vue/CSS.
- [ ] Introduzir erro de compilação no fixture, observar o erro e corrigi-lo; exigir recuperação automática da versão mais recente, sem build/restart manual.

#### 3.2 Prova obrigatória de encerramento

- [ ] Executar o harness com Air `v1.61.7` e filho que termina 8 segundos após o primeiro sinal; registrar a cronologia e falhar se a substituta começar antes de sua saída.
- [ ] Repetir com edições sucessivas durante o drain e com parada do supervisor/container; exigir um único sinal de término por filho, término antes da saída do supervisor e ausência de filhos ativos após cleanup.
- [ ] Rodar o teste dedicado via `WZAP_TEST_AIR_BIN=/go/bin/air go test ./docker -run TestDevReload -count=1` em ferramenta/container dev com Go e Air disponíveis, usando diretórios temporários independentes; se Air não existir no ambiente de testes comum, registrar SKIP ali e executar obrigatoriamente esta prova no ambiente dev.

#### 3.3 Proteção somente se necessária

- [ ] Se todos os cenários nativos passarem, registrar a evidência e não criar wrapper; a tarefa fica satisfeita pela prova e ausência de proteção adicional.
- [ ] Se algum cenário falhar, registrar RED e criar somente `docker/dev-go.sh` ou proteção equivalente mínima para serializar execuções e aguardar o filho exato terminar, sem repetir seu sinal, sem lógica própria de build/watch, sem arquivos PID e sem matar processos por nome.
- [ ] Repetir o mesmo harness até GREEN e conferir a parada Docker com prazo de 20 segundos; cobrir também compilação inválida corrigida e eventual corrida de limpeza do binário, alterando opções do Air apenas com evidência.

**Ponto de commit futuro:** recarga/encerramento e testes, com prefixo `fix(dev):`; a evidência deve explicar se houve necessidade de proteção.

## 4. Compose — tarefas 4.1–4.2

### Task 6: Sobreposição Compose e troca preservada (4.1–4.2)

**Arquivo:** `docker-compose.dev.yml`; preservar o empacotamento no `Dockerfile` de produção e a infraestrutura declarada no Compose base.

#### 4.1 Sobreposição do mesmo serviço

- [ ] Substituir o Compose dev independente pela sobreposição descrita no design, mantendo apenas `wzap`, `manager-dev` e volumes de cache necessários; herdar o projeto e os volumes de dados do base.
- [ ] Usar imagem `wzap:dev-live-reload`, sources montados, Go healthcheck `/src/tmp/wzap healthcheck`, `WZAP_MANAGER_DEV_URL=http://manager-dev:3000` e prazo de parada de 20 segundos; manager sempre inicia no comando dev, com instalação pelo lockfile e sem porta host.
- [ ] Rodar `docker compose -f docker-compose.yml -f docker-compose.dev.yml config`, inspecionar serviço/projeto, comando, imagem, mounts, healthcheck e portas, e confirmar um único serviço Go; incluir override local entre base/dev quando existente sem expor segredos na evidência.

#### 4.2 Troca real e preservação

- [ ] Registrar IDs da infraestrutura e nomes dos volumes; identificar e encerrar somente um eventual Go legado antes da troca, preservando seus volumes e os demais serviços.
- [ ] Ativar dev substituindo o serviço `wzap`, abrir o manager em `8081` e comparar infraestrutura/volumes; não iniciar o mesmo banco com outro projeto/serviço Go durante a validação.
- [ ] Alterar temporariamente Go e frontend, confirmar o novo comportamento e comparar IDs dos containers da aplicação/manager antes e depois das edições; restaurar o código de prova.
- [ ] Exercitar o retorno ao compilado encerrando primeiro Go dev e Nuxt, conferir ausência de duas execuções e dados preservados, e voltar ao modo necessário para concluir a validação integrada.

**Ponto de commit futuro:** sobreposição Compose e documentação operacional correspondente, com prefixo `feat(dev):`.

## 5. Gates e documentação — tarefas 5.1–5.2

### Task 7: Gates, validação integrada e documentação (5.1–5.2)

#### 5.1 Validação integrada

- [ ] Executar `gofmt` nos Go alterados, `go vet ./...`, golangci-lint `v2.13.2`, `go test ./... -count=1` e `go build ./...`; rodar explicitamente a prova Air no ambiente com a ferramenta real, mesmo se a suite comum a omitir por ausência de Air.
- [ ] Executar `pnpm --dir manager test`, `lint`, `typecheck` e `build`; aplicar os comandos de freshness Swagger do AGENTS somente se annotations tiverem mudado.
- [ ] Usar `agent-browser` no UI atualizado em desktop e mobile para login, navegação direta/refresh e atualização Vue/CSS; recarregar Go automaticamente, confirmar reconexão HMR e comprovar que uma nova edição frontend funciona após a recarga.
- [ ] Capturar screenshots e observações sem credenciais/segredos; parar Nuxt temporariamente para comprovar `503` e recuperá-lo para comprovar o retorno do painel.
- [ ] Integração Postgres somente com `WZAP_TEST_DATABASE_URL` definido e alcançável, DB terminado em `_test` e schemas isolados; NATS somente com a URL de teste alcançável, registrando SKIP quando faltar condição e sem alegar execução.

#### 5.2 Produção e instruções de uso

**Arquivos:** `README.md`, comandos de setup em `AGENTS.md` e `manager/AGENTS.md`, preservando conteúdo alheio ao escopo.

- [ ] Construir a imagem de produção com seu Dockerfile atual e validar manager/rota interna/asset embutidos após encerrar Go dev e Nuxt, usando uma única réplica; não alterar a arquitetura de produção para facilitar o teste.
- [ ] Documentar os comandos base+dev e base+override-local+dev, inicialização do manager, retorno ao compilado, logs e indisponibilidade breve durante recarga Go; retirar referências ao projeto dev separado e à geração manual do manager servido no modo dev.
- [ ] Distinguir edição comum de código/estilo de mudança de dependência, ambiente ou Dockerfile, que pode exigir reinstalação/reaplicação; conferir cada comando contra a configuração resolvida e a troca validada.
- [ ] Depois da implementação, produzir `verify.md` com os sete checks e `retrospective.md` baseada nas evidências, revisar o conjunto e rodar `openspec validate enable-dev-live-reload --strict`; arquivar somente na etapa final apropriada.

**Ponto de commit futuro:** documentação/evidências com prefixo `docs(specs):`; commits, publicação de PR e arquivamento não fazem parte da criação desta proposta.
