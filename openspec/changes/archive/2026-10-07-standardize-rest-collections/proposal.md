## Why

As coleções REST usam `items` e, em vários casos, repetem a entidade dentro de cada item, dificultando o consumo e deixando o nome da coleção implícito. Campos opcionais sem configuração também aparecem como `null`; padronizar esses formatos torna o contrato mais direto e coerente entre listagem e detalhe. O transporte HTTP concentra o registro das rotas e contratos amplos de serviços em um pacote que cresceu com grupos, contatos, canais, status e integrações. A migração é integral, sem adapters, aliases ou bridges para a estrutura substituída. A mesma change passa a reunir a padronização JSON e a migração completa para Chi com organização por recurso, conforme a decisão aprovada, para tornar manutenção e testes mais locais.

## What Changes

- Migrar todas as rotas HTTP para Chi v5, usando `net/http` como servidor, autorização explícita e um contrato final de roteamento/idempotência sem camada de compatibilidade.
- Separar o transporte em pacotes por recurso, com interfaces definidas pelos consumidores e infraestrutura compartilhada mínima, sem dependência dos recursos no pacote que monta o servidor.
- Chi v5.3.2 será a única nova dependência direta de produção; manter a versão fixada e preservar o gerador Swaggo atual.
- Nas rotas de mensagens, usar `instance` para a referência da instância e `id` para a mensagem: `/instances/{instance}/messages/{id}`, com resolução independente dos parâmetros e a mesma URL concreta.
- **BREAKING**: substituir `data.items` por coleções nomeadas em todas as dez operações encontradas: `instances`, `users`, `groups`, `messages`, `channels`, `statuses`, `contacts` e `blocked_jids`, conforme a matriz do design.
- **BREAKING**: colocar os campos do recurso diretamente em cada elemento da coleção, removendo os wrappers por item `instance`, `user`, `group`, `message` e `channel`.
- Manter ordem consistente nas structs para legibilidade, sem transformar a ordem das propriedades JSON em requisito de contrato ou teste.
- **BREAKING**: omitir campos opcionais sem valor/configuração nos DTOs compartilhados com essas coleções e omitir `next_cursor` quando não houver próxima página, mantendo-o junto da coleção dentro de `data`.
- Modelar subobjetos opcionais com ponteiros, omitir `settings` quando nenhum subbloco estiver disponível e manter `integration.webhook` obrigatório mesmo desativado.
- Garantir slices obrigatórios não nil e sem `omitempty`, preservando `[]`, flags `false`, contadores `0` e duração `"0"`; usar ponteiros para distinguir ausência de valores zero válidos e `*time.Time` para datas opcionais.
- **BREAKING**: omitir datas de atualização de grupos e canais quando desconhecidas, sem inventar timestamps.
- Usar `channels` para metadados de canais, `statuses` para publicações próprias de status e `messages` também na consulta de atualizações de canal, que retorna DTOs de mensagem.
- Garantir equivalência de representação entre listagens e detalhes para o mesmo estado e contexto, com testes estruturais independentes da ordem textual do JSON.
- Adequar Manager, testes de contrato, documentação e Swagger ao formato final.

## Capabilities

### New Capabilities

- `wzap-http-routing`: roteamento completo, limites de acesso e idempotência do contrato Chi final.

### Modified Capabilities

- `wzap-response-contract`: coleções nomeadas com itens diretos, opcionais ausentes e erros estruturados opcionais.
- `wzap-instances`: coleção `instances` e agregação com omissão de blocos indisponíveis.
- `wzap-outbound-messaging`: ausência dos campos opcionais de envio e erro sem valor, inclusive no aceite.
- `wzap-manager`: consumo das coleções nomeadas e tolerância a propriedades opcionais ausentes.

## Impact

Afeta a montagem de todas as rotas, middleware, DTOs, agregadores e handlers em `internal/httpapi`, seus testes de JSON/Swagger, `go.mod`/`go.sum`, `manager/app/types/api.ts`, composables e telas consumidoras, `README.md`, `docs/` gerados e a orientação de contrato em `AGENTS.md`. Clientes externos precisam migrar nomes de coleções, acesso aos itens e tratamento de ausência; backend e Manager devem ser entregues juntos. Chi é a única dependência direta de produção nova autorizada; não exige migrações de banco.

## Out-of-Scope

Novos endpoints, remodelagem de comandos independentes dessas representações, payloads de escrita, eventos NATS/webhooks, persistência, mudanças de autorização, alterações nos algoritmos de ordenação/paginação, paginação de servidor para instâncias, aliases de compatibilidade para `items` e conversão de respostas idempotentes já armazenadas. Não adotar Huma, Gin, Echo, Fiber, OpenAPI 3 ou geração de código; não reorganizar os serviços de domínio, repositórios e workers que não pertencem ao transporte HTTP; não alterar `.github/workflows/`.
