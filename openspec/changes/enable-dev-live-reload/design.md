## Context

Ver `proposal.md` para a motivação. Hoje, o Compose principal usa o projeto `wzap`, serve o binário compilado em `8081` e mantém Postgres, NATS, MinIO e mídia em volumes. O arquivo dev é independente (`wzap-dev`), repete infraestrutura, publica Go em `8083` e deixa Nuxt em um perfil opcional; o painel servido pelo Go usa assets gerados em disco.

O manager já usa Nuxt com `ssr: false`, base `/manager/`, API na mesma origem e proxy Vite para uso standalone. A produção gera e embute os assets no binário. O Go tem encerramento limitado a 10 segundos, aborto no segundo sinal e bloqueio de réplica por banco. Air está fixado em `v1.61.7` no Dockerfile dev.

## Goals / Non-Goals

**Goals:** refletir Go, Vue e CSS locais automaticamente; preservar a entrada pública e os dados existentes; comprovar que um reinício não sobrepõe execuções nem encurta o encerramento do serviço.

**Non-Goals:** trocar router, mover domínio para Nuxt, separar workers, alterar o ciclo de vida do Go ou introduzir infraestrutura adicional; transformar Air em um supervisor próprio. Os limites funcionais estão em `proposal.md`.

## Decisions

### 1. Go continua como entrada HTTP

`config.Config.ManagerDevURL` recebe `WZAP_MANAGER_DEV_URL`, vazio por padrão. Aceitar somente URL absoluta HTTP/HTTPS com host e path vazio ou `/`, sem userinfo, query ou fragmento; erro de configuração deve citar a variável sem reproduzir o valor. No Compose dev, usar `http://manager-dev:3000`.

Construir o handler do manager uma única vez na composição HTTP: encaminhamento quando o endereço dev estiver configurado, atendimento estático existente nos demais casos. Usar `httputil.ReverseProxy` com `Rewrite`, preservar o Host de entrada e o caminho completo, inclusive o prefixo `/manager/`, RawPath e query, e reconstruir headers de encaminhamento com a API padrão. Preservar o upgrade WebSocket da biblioteca padrão, sem dependência extra.

O handler dev redireciona `/manager` para `/manager/` com `301` e conserva a query. Falha de transporte retorna texto genérico com `503`; não tentar fallback para disco ou embed e não registrar URLs completas, query, cookies ou tokens. Não encaminhar REST, probes ou Swagger ao Nuxt. Sem endereço dev, preservar `Handler()`, `WZAP_MANAGER_DIR`, embed e placeholder existentes.

Alternativas consideradas: Nuxt como entrada mudaria a responsabilidade pública da API; um proxy externo acrescentaria outro serviço sem necessidade; servir `.output/public` durante desenvolvimento manteria a geração manual.

### 2. Nuxt dev independente, com HMR pela origem pública

Manter base `/manager/`, SSR desligado e `apiBaseUrl` vazio. O serviço `manager-dev` usa Node 24 e o pnpm fixado em `manager/package.json`, instala pelo lockfile e executa Nuxt em `0.0.0.0:3000` na rede interna. Retirar perfil opcional e publicação de porta no host.

Configurar a porta pública do cliente WebSocket via `NUXT_DEV_HMR_CLIENT_PORT=8081`, aplicada somente em desenvolvimento à opção `vite.server.ws.clientPort` da versão instalada; preservar os paths derivados da base Nuxt sem prefixar `/manager/` duas vezes. Confirmar no navegador que não ocorre fallback para conexão direta à porta interna.

Preservar o proxy Vite existente para desenvolvimento standalone, com `NUXT_DEV_PROXY_TARGET=http://wzap:8080` dentro do Compose e o padrão local `8081`; não adicionar regras Nitro que confundam `/manager/instances` com a coleção REST. Montar os fontes e manter `node_modules`, `.nuxt` e demais saídas/cache separados quando necessário para não interferir nas ferramentas do host.

Alternativa considerada: uma porta pública própria para Nuxt exigiria outro endereço e não comprovaria o mesmo fluxo de cookies e HMR da entrada canônica.

### 3. Air nativo primeiro; proteção condicionada à prova

Manter Air `v1.61.7`, build Go direto, debounce de 200 ms, `send_interrupt = true`, `full_bin = "exec ./tmp/wzap serve"`, `kill_delay = "12s"` e `stop_grace_period: 20s`. Observar `.go`, `go.mod`, `go.sum` e SQL de migrações embutidas; incluir Go gerado em `docs` e excluir testes, caches, `tmp`, dependências frontend e assets Nuxt. Vue/CSS não devem disparar build Go.

Não atribuir garantia de serialização somente ao `kill_delay`: [a versão fixada possui espera interna de 5 segundos](https://github.com/air-verse/air/blob/v1.61.7/runner/engine.go#L610-L633), e [o encerramento Linux aguarda o delay antes de SIGKILL](https://github.com/air-verse/air/blob/v1.61.7/runner/util_linux.go#L11-L26). Na implementação, executar primeiro uma prova com Air real e um filho controlado que encerra em 8 segundos; a prova roda em diretório temporário, sem `serve` real, banco ou metadados do serviço ativo.

Critério determinístico: se Air nativo passar todos os cenários de recarga, edições sucessivas, erro de build e parada, não criar wrapper. Se falhar, permitir somente uma proteção dev mínima que serialize a execução substituta e mantenha o supervisor vivo até o filho terminar, sem reenviar o sinal já entregue. Ela não observa arquivos nem compila; não mantém protocolo de arquivos PID e não procura processos pelo nome. Repetir os mesmos testes. Ajustar `stop_on_error`/limpeza apenas se a prova mostrar interferência com a recuperação ou com o binário recém-gerado e registrar o motivo.

Alternativas consideradas: wrapper obrigatório em três modos adicionaria lógica antes de provar sua necessidade; reduzir o timeout Go esconderia a falha; atualizar Air sem comprovar seu comportamento não resolveria a garantia exigida.

### 4. Mesmo projeto e mesmo serviço Go nos dois modos

Converter `docker-compose.dev.yml` em uma sobreposição do arquivo base: herdar projeto `wzap`, serviço `wzap`, infraestrutura, rede, porta `8081` e volumes de dados. Sobrescrever apenas imagem dev (`wzap:dev-live-reload`), build pelo Dockerfile dev, ambiente específico, mounts/cache, healthcheck (`/src/tmp/wzap healthcheck`) e prazo de parada; acrescentar `manager-dev`. Não declarar outro serviço Go ou outro nome de projeto dev.

Isso substitui a configuração do mesmo container de aplicação ao alternar os modos e evita duas instâncias pelos comandos suportados. Manter o bloqueio de réplica como proteção adicional para usos externos ao fluxo documentado. Usar volumes de cache dev apenas para compilação/dependências e preservar os nomes efetivos dos volumes de dados existentes.

O comando padrão dev será `docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build wzap manager-dev`. Quando houver `docker-compose.override.yml` local, incluí-lo entre o base e o dev, preservando suas portas e configurações; comandos explícitos com `-f` não o carregam automaticamente. Antes de aplicar na máquina existente, conferir configuração e infraestrutura ativa para não recriar dependências apenas por diferença de portas ou overrides. Não acrescentar serviço de ferramentas ou targets de imagem sem necessidade da change.

Alternativas consideradas: dois serviços com profiles poderiam ser iniciados juntos e duplicariam configuração; um projeto dev separado não reutilizaria a infraestrutura e os volumes do projeto principal.

## Risks / Trade-offs

- [Risk] Air permite prosseguir após 5 segundos antes de um drain de 8–10 segundos terminar -> Mitigation: prova obrigatória com processo controlado e proteção mínima somente se necessária; o teste exige a saída real do filho antes da próxima execução e antes da saída do supervisor.
- [Risk] HMR tenta alcançar a porta interna ou duplica a base -> Mitigation: verificar o WebSocket `101` pela porta `8081`, payloads nos dois sentidos e uma nova edição após recarga Go.
- [Risk] Compose mescla imagem, mounts, portas ou healthcheck incorretamente -> Mitigation: inspecionar `docker compose ... config`, usar imagem dev distinta, herdar a porta base e comparar volumes/containers de infraestrutura antes e depois da troca.
- [Risk] Um projeto dev legado ainda executa Go no mesmo banco -> Mitigation: identificá-lo e parar somente sua aplicação antes da primeira ativação, sem remover volumes; não executar a validação com segunda réplica.
- [Risk] Recarga Go causa uma breve interrupção de HTTP, HMR e sessões WhatsApp -> Mitigation: manter o encerramento e a restauração atuais; validar reconexão do painel e documentar a interrupção esperada.
- [Risk] Dependências, Dockerfile ou variáveis de ambiente mudam sem reaplicação -> Mitigation: documentar reinstalação/recriação quando esses insumos mudarem; a promessa de recarga automática cobre alterações normais de código e estilos.

## Migration Plan

1. Durante apply, trabalhar em `.worktrees/enable-dev-live-reload` na branch `codex/enable-dev-live-reload`, preservando alterações alheias; preparar o contrato Go antes do ajuste Nuxt.
2. Registrar a aplicação ativa, o projeto efetivo, mapeamentos locais e nomes dos volumes; conferir a sobreposição resolvida antes de qualquer troca de serviço.
3. Se existir Go legado de `wzap-dev`, encerrá-lo primeiro, sem `down -v`; ativar o novo modo dev substituindo somente a aplicação e acrescentando o manager.
4. Validar recarga e troca de modo preservando containers de infraestrutura e volumes; produzir evidências de navegador e checks somente durante implementação.
5. Para retorno, parar `wzap` e `manager-dev` usando o conjunto de arquivos dev e subir `wzap` pelo Compose base, com o override local quando existente, sem excluir volumes; verificar que a imagem compilada atende o manager embutido sem Nuxt em execução.

Sem migração de dados, novas dependências de aplicação ou alteração de CI. `verify.md` e `retrospective.md` serão produzidos após apply; arquivamento fica para a etapa final do workflow.
