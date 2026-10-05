## Why

O Swagger pede novamente a chave já informada no Authorize e mostra campos técnicos dispensáveis. O usuário também pediu que a listagem de instâncias deixe de exigir navegação por páginas.

## What Changes

- Usar somente o esquema de segurança apikey no Swagger, sem parâmetro manual duplicado nas operações.
- Remover o campo de entrada X-Request-Id da documentação; continuar gerando e retornando o identificador automaticamente.
- **BREAKING**: GET /instances retorna todas as instâncias autorizadas em `{"data":{"items":[...]}}`, sem `limit`, `cursor` ou `next_cursor`; parâmetros antigos são ignorados.
- Adaptar stats, restauração de sessões, importador Chatwoot e Manager ao contrato interno de listagem completa.

## Capabilities

### New Capabilities

Nenhuma.

### Modified Capabilities

- `wzap-operations`: autorização única no Swagger e ausência de campos manuais de correlação.
- `wzap-instances`: listagem completa e ordenada com o escopo atual de autorização.
- `wzap-manager`: listagem, métricas e uso por conta obtidos em uma única consulta de instâncias.

## Impact

Transporte REST, serviço/repositório de instâncias, consumidores internos, clientes Nuxt, documentação gerada e testes. Sem novas dependências de runtime. Clientes externos que consultavam next_cursor precisam usar data.items diretamente.

## Out-of-Scope

Paginação de mensagens, grupos e newsletters; paginação visual das tabelas; alterações de autenticação, RBAC, webhooks, eventos ou rastreamento de requisições; correções locais ainda não commitadas pelo usuário.
