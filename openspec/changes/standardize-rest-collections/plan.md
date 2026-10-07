# Chi e coleções REST — Implementation Plan

> **For agentic workers:** usar superpowers:subagent-driven-development e executar task-by-task com TDD. O usuário autorizou um especialista backend; coordenação, Manager e documentação pertencem ao agente principal.

**Goal:** migrar integralmente o transporte para Chi, separar recursos em pacotes reais e entregar o contrato JSON e seus consumidores na mesma change.

**Architecture:** httpapi compõe routers Chi; recursos implementam handlers com interfaces mínimas e consomem core/representation. Serviços de domínio, armazenamento e workers mantêm suas fronteiras. Nenhuma camada de compatibilidade, bridges de testes ou geração OpenAPI.

**Tech Stack:** Go 1.26, Chi v5 estável fixado, encoding/json, Swaggo v1.16.6/Swagger 2; Manager Nuxt 4/Vue 3/TypeScript/pnpm.

**Spec:** proposal.md, design.md e specs/ desta change; tasks.md define o escopo.

## Global Constraints

- Única nova dependência direta de produção: github.com/go-chi/chi/v5.
- Não alterar migrações, eventos, payloads de escrita ou .github/workflows/.
- Coleções obrigatórias e arrays aninhados: [] não nil e sem omitempty.
- Opcionais ausentes omitidos; preservar false, 0, "0" e arrays obrigatórios vazios.
- Segredos e campos internos fora dos DTOs públicos.
- Usar .worktrees/standardize-rest-collections; não interferir nas alterações da árvore principal.
- Sem adapters de padrões substituídos, formatos simultâneos ou aliases/bridges.
- Postgres somente _test com schema isolado; integração exige serviço acessível e env definida.

## Review Focus

- Referência de nome percent-encoded identifica o alvo correto sem consultar nomes estrangeiros (6.5).
- HEAD e Allow refletem o roteamento final, com fallbacks envelopados e autenticação no namespace privado (6.1/6.2).
- Falha parcial da agregação não omite os demais blocos nem esconde timer "0" (2.2).
- Idempotência não executa a operação em fingerprint diferente nem consulta replay sem autorização (2.5/6.5).
- Manager com settings/data/erro ausentes continua navegável e encerra paginação sem cursor (3.2).

## Ownership e arquivos

- Especialista backend: go.mod/go.sum; internal/httpapi/server.go, routes.go, swagger.go; core/, representation/ e recursos instances/, messages/, groups/, contacts/, channels/, chats/, statuses/, profile/, users/, authsession/, media/, chatwoot/; testes Go; docs/ gerados.
- Coordenador: manager/, README.md, AGENTS.md e arquivos desta change. Nunca dois escritores no mesmo arquivo.
- Remover arquivos monolíticos de handlers depois de mover seus conteúdos; não deixar wrappers raiz. Testes de funções privadas acompanham o pacote; integração usa a API pública.

## 1. Preparação (tasks 1.1, 6.1)

- [x] Preparar embed .output/public no worktree a partir do build disponível ou build Manager, sem versionar artefatos gerados; instalar frontend com lockfile.
- [x] Rodar baseline `go test ./internal/httpapi -count=1` e `pnpm --dir manager test`; registrar falhas ambientais separadas de falhas do contrato.
- [x] Inventariar cada Handle/HandleFunc de server.go e caminhos especiais; usar a matriz como referência para chi.Walk final.
- [x] Acrescentar cenários do contrato final: 404 raiz, 401 namespace privado, 405/Allow, HEAD, sessão, arquivos estáticos e webhook público. Confirmar falhas pertinentes antes de alterar routers.

## 2. DTOs e contrato (tasks 1.2, 2.1–2.5)

**Arquivos:** dto.go, instances.go, groups.go, newsletters.go, parity_reads.go, users.go, status.go, chatwoot.go e testes; destino final representation/ e DTOs específicos dos recursos.

- [x] Escrever testes estruturais para as dez operações da matriz; fonte nil => array [], ausência de items/wrapper, cursor junto da coleção e omitido ao final. Rodar `go test ./internal/httpapi/... -count=1` para confirmar falhas esperadas.
- [x] Implementar coleções diretas e construtores com make/nil normalization; usar o mesmo mapper nos detalhes e listas.
- [x] Escrever teste de settings ausente/parcial, timer "0", webhook disabled e Chatwoot persistido disabled; verificar presença e zeros com maps/RawMessage.
- [x] Implementar ponteiros/omitempty campo a campo conforme catálogo do design; timestamps opcionais via *time.Time; nenhum encoder customizado.
- [x] Testar participantes/events/privacy/ignored_jids vazios, erro sem data, mensagem retry_count=0, aceite sem media_id e exclusão de segredos.
- [x] Rodar testes locais; não usar comparações dependentes da ordem textual JSON, exceto replay literal da mesma requisição.

## 3. Core e recursos (tasks 6.2–6.5)

**Interfaces:** raiz mantém `New(cfg config.Config, log zerolog.Logger, deps Deps) *http.Server`. Recursos expõem `Register` recebendo chi.Router e dependências específicas. Core expõe middleware de autenticação/autorizações e acesso tipado ao alvo resolvido no contexto. Representation expõe DTOs/mapeadores de recursos públicos reutilizados por handlers.

- [x] Fixar Chi v5 estável em go.mod/go.sum; extrair core para envelopes, body, limites, auth/RBAC, lookup, idempotência e erros; extrair representation sem import de handlers.
- [x] Migrar instâncias e usuários para os primeiros pacotes reais e verificar compilação sem ciclos; suas interfaces incluem somente métodos consumidos.
- [x] Mover cada operação de mensagens/grupos/contatos/canais/chats/status/perfil/media/chatwoot/authsession, incluindo parity_*; colocar requests, responses, handlers e tests próximos da responsabilidade.
- [x] Substituir toda composição HTTP por Chi; root usa GetHead, fallback JSON e registro público explícito; namespaces privados autenticam o router inteiro.
- [x] Implementar middleware de alvo: lookup UUID/nome, RBAC e contexto canônico; handlers/idempotência usam esse alvo. Usar padrão nativo Chi para fingerprint, sem normalização alternativa.
- [x] Declarar rotas de mensagens com o ancestral `instance` e detalhe com `id`, usando parâmetro explícito no resolver; conferir identificação de instância e mensagem diferentes em HTTP e os parâmetros `instance`/`id` no Swagger.
- [x] Migrar testes locais e fixtures para seus pacotes; integração testa httpapi.New e requisições HTTP, sem bridges ou aliases para API privada.
- [x] Rodar `go test ./internal/httpapi/... -count=1`, `go build ./...`; revisar inventário e procurar ServeMux/instanceMux/envelopeFallback de produção para confirmar remoção.

## 4. Manager (tasks 3.1–3.3)

**Arquivos:** manager/app/types/api.ts, composables/useInstances.ts, useAccounts.ts, useOverview.ts, useMessages.ts, useInstanceGroups.ts, useInstanceChannels.ts, useInstanceProfile.ts, useInstanceChatwoot.ts, componentes/páginas que consomem settings/connection e manager/test(s)/ fixtures existentes.

- [x] Atualizar fixtures/tests para data.instances/users/groups/messages/channels/statuses/contacts/blocked_jids diretos; testar opcionais omitidos, timer "0" e ausência de next_cursor. Executar `pnpm --dir manager test` e confirmar falhas relacionadas antes de modificar consumidores.
- [x] Atualizar tipos das coleções e propriedades opcionais; separar presença de defaults usados na UI, sem mudar inputs de escrita.
- [x] Substituir consumo de page.items/wrappers pelos recursos finais; usar optional chaining/fallback de disponibilidade e cursor opcional.
- [x] Rodar `pnpm --dir manager test`, `lint`, `typecheck`, `build` e corrigir falhas. Conferir os fluxos afetados conforme testes UI existentes e inspeção do código/renderização quando disponível.

## 5. Swagger e documentação (tasks 4.1, 4.2, 6.6)

- [x] Atualizar annotations/tipos e testes Swagger para os schemas finais, campos required e opcionais; cobertura usa chi.Walk sobre operações reais.
- [x] Regenerar `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs`; repetir geração e confirmar determinismo comparando artefatos antes/depois.
- [x] README/AGENTS descrevem Chi, pacotes, recursos diretos e opcionais omitidos; exemplos mostram estado final sem alternativas históricas. Registrar BREAKING na change e entrega.
- [x] Executar testes Swagger e `openspec validate standardize-rest-collections --strict`.

## 6. Gates e entrega (tasks 5.1–5.3)

- [x] Executar gofmt, go vet ./..., golangci-lint v2.13.2, go test ./... -count=1 e go build ./...; executar gates completos Manager uma vez no estado final.
- [x] Identificar Postgres/NATS acessíveis sem iniciar réplica; executar integração somente com env apropriada e DB _test/schema isolado ou registrar SKIP com motivo.
- [x] Revisar escopo, dependências acíclicas, rotas completas, privacidade, ausência de wrappers/adapters e todos os cenários JSON; corrigir achados e repetir somente checks afetados.
- [x] Marcar tasks apenas após evidência, produzir verify.md (sete checks) e retrospective.md; não arquivar antes da verificação.

Pontos de commit reviewable: contrato/DTOs; core/Chi/recursos; Manager; Swagger/documentação; verificação final. Commits usam feat(api):/docs(specs): e identificam BREAKING. Não publicar/mergear sem escopo explícito adicional.

## Evidências durante apply

- Baseline backend: `go test ./internal/httpapi -count=1` passou em 3.900s antes da migração.
- RED Manager: cinco cenários novos de coleções diretas falharam por acessos a items; os dois testes existentes passaram. GREEN: sete testes passaram depois da mudança.
- Manager: typecheck e build passaram; lint passou com dois avisos em DataTableToolbar.vue/PageState.vue não alterados. Typecheck foi repetido após completar os tipos das dez operações.
- QA Luna via build estático e mock local isolado, sem réplica ou dados reais: settings/chatwoot_config ausentes; timer configurado "0"; mensagens diretas com retry_count=0 e opcionais omitidos; detalhe navegável e sem erros de console; botão Load more ausente sem next_cursor. Servidor e navegador encerrados.
- Cinco agentes Luna auxiliaram revisão Manager, documentação, serialização JSON, roteamento/segurança e QA de UI em ondas. Achados de Chatwoot opcional, alvo de webhook por nome e parâmetros escapados foram encaminhados ao especialista backend.
- Decisão adicional do usuário: `GET /instances/{instance}/messages/{id}` no Go e Swagger; parâmetros distintos, URLs concretas e representação `data.message.id` preservadas.

- Backend RED reportado pelo especialista antes da implementação: opcionais null em SerializationPresenceMatrix, root desconhecido 401 em vez de 404, Allow ausente e matriz de coleções com items/wrappers. Regressão de canal escapado reproduziu 123%40newsletter no serviço e passou após PathParam. Saídas iniciais observadas na sessão; sem arquivo de log separado preservado.
- GREEN final HTTP em Go 1.26.0: `go test ./internal/httpapi/... -count=1`, incluindo HEAD em servidor HTTP real, equivalência lista/detalhe, dez coleções e required/optional Swagger; log `/tmp/wzap_http_final.log`.

- Gates finais Go 1.26.0 e lint 2.13.2 passaram; integração completa em wzap_test/schemas isolados e broker NATS temporário passou. Swagger repetido com hashes idênticos. Manifest: `/tmp/wzap_chi_gates_final.json`; resultados persistidos em verify.md. Container NATS encerrado após testes.
- Revisão final encerrou gaps de HEAD real, contacts nil, equivalência lista/detalhe e required/optional Swagger. verify.md (sete checks PASS) e retrospective.md produzidos; strict validation passou. Change não arquivada; implementação no worktree sem commit/merge/deploy.
