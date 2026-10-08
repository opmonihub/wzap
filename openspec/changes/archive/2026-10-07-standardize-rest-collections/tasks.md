## 1. Preparação e testes de contrato

- [x] 1.1 Criar `plan.md` com micro-passos TDD por ID destas tarefas, conferir o worktree `.worktrees/standardize-rest-collections`, preparar o embed do Manager e registrar o baseline dos testes adicionando somente a dependência Chi autorizada e preservando arquivos alheios ao escopo.
- [x] 1.2 Atualizar os testes HTTP/DTOs para as dez coleções nomeadas, elementos diretos, arrays obrigatórios vazios, opcionais ausentes e zeros válidos, confirmando falhas de contrato esperadas com `go test ./internal/httpapi/... -count=1` antes da implementação.

## 2. Contrato REST e DTOs Go

- [x] 2.1 Substituir `items` e wrappers individuais nas dez operações pela matriz de coleções do design e normalizar slices obrigatórios não nil sem `omitempty`, verificando coleções vazias e preenchidas com `go test ./internal/httpapi/... -count=1`.
- [x] 2.2 Modelar `settings` e `chatwoot_config` como objetos opcionais, manter webhook obrigatório desativado e preservar agregação independente e concorrência limitada, verificando settings ausente/parcial, timer `"0"` e configurações persistidas desativadas nos testes HTTP de instância.
- [x] 2.3 Atualizar campos compartilhados de conexão, erros, mensagens, aceite, grupos, canais, perfil e Chatwoot conforme o catálogo do design, verificando omissão dos opcionais e preservação de timestamps obrigatórios, flags, contadores, defaults e arrays vazios com `go test ./internal/httpapi/... -count=1`.
- [x] 2.4 Aplicar `next_cursor` opcional junto da coleção e garantir equivalência estrutural entre listagem e detalhe para o mesmo estado e contexto, verificando páginas intermediárias/finais e representações equivalentes sem comparar ordem textual do JSON.
- [x] 2.5 Preservar exclusão de segredos e campos internos, escopos de acesso e replay integral sem conversão, verificando os testes de privacidade, autorização e idempotência de `internal/httpapi`.

## 3. Manager

- [x] 3.1 Atualizar interfaces e composables para coleções diretas nomeadas, arrays obrigatórios e propriedades opcionais sem obrigatoriedade de null, verificando os novos tipos com `pnpm --dir manager typecheck`.
- [x] 3.2 Atualizar listas, overview e detalhes para configurações, erros, fotos e datas ausentes, zeros válidos e fim de paginação sem cursor, verificando fixtures e fluxos afetados com `pnpm --dir manager test`.
- [x] 3.3 Preservar formatos de escrita e interação das telas consumidoras e confirmar funcionamento da UI com `pnpm --dir manager lint`, `pnpm --dir manager typecheck`, `pnpm --dir manager build` e verificação dos fluxos afetados.

## 4. Documentação e Swagger

- [x] 4.1 Atualizar README, comentários públicos e a orientação de contrato em AGENTS.md para coleções, ausência de opcionais, arrays obrigatórios vazios e quebra BREAKING, conferindo os exemplos contra a matriz HTTP final.
- [x] 4.2 Atualizar annotations e testes de Swagger, regenerar `docs/` com `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs` e confirmar schemas de coleções obrigatórias/opcionais e geração determinística sem alterações adicionais.

## 6. Chi e organização integral do transporte

- [x] 6.1 Registrar o inventário completo método/caminho e escrever testes do contrato Chi final para HEAD, 404, 405/Allow, rotas públicas e namespaces autenticados, confirmando as diferenças esperadas antes da implementação.
- [x] 6.2 Fixar versão estável de Chi v5 e migrar todos os routers HTTP de produção, verificando as operações do inventário, o servidor padrão e a cadeia de middleware com os testes HTTP.
- [x] 6.3 Extrair core e representation com responsabilidades limitadas e dependências unidirecionais, definindo interfaces mínimas por consumidor e verificando ausência de ciclos com go build ./....
- [x] 6.4 Mover handlers reais, DTOs específicos e registros de todas as operações para pacotes de instâncias, mensagens, grupos, contatos, canais, chats, status, perfil, usuários, autenticação, mídia e Chatwoot, distribuindo parity_* por recurso sem wrappers de produção e verificando go test ./internal/httpapi/... -count=1.
- [x] 6.5 Resolver e autorizar o alvo de instância no contexto com parâmetro explícito por rota, declarar o detalhe de mensagem como `/instances/{instance}/messages/{id}`, usar o padrão nativo Chi na identificação idempotente e verificar escopos, aliases de nome válidos, conflito de fingerprint e replay literal da mesma operação, sem adaptadores para versões substituídas ou alteração dos registros de banco.
- [x] 6.6 Migrar testes locais aos pacotes dos recursos, manter integração sobre HTTP público sem bridges e usar chi.Walk para conferir todas as operações do Swagger, verificando cobertura e freshness dos arquivos gerados.

## 5. Verificação e entrega

- [x] 5.1 Verificar a implementação integrada com gofmt limpo, `go vet ./...`, golangci-lint v2.13.2, `go test ./... -count=1`, `go build ./...` e os gates test/lint/typecheck/build do Manager, registrando os resultados e corrigindo falhas antes da conclusão.
- [x] 5.2 Executar integração Postgres somente em DB acessível com nome `_test`, schema isolado e `WZAP_TEST_DATABASE_URL` definido e integração NATS com serviço acessível e `WZAP_TEST_NATS_URL` definido, registrando explicitamente SKIP quando as condições não forem atendidas sem alegar execução.
- [x] 5.3 Produzir `verify.md` após apply e `retrospective.md` antes da PR, realizar revisão do conjunto e validar `openspec validate standardize-rest-collections --strict`, mantendo os componentes da entrega conjunta e deixando o arquivamento para depois da verificação.
