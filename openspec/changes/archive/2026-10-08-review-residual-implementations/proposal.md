## Why

A revisão solicitada das implementações recentes encontrou falhas que não são exercitadas pelas suites atuais, apesar de todos os gates da baseline passarem. Credenciais continuam utilizáveis após remoção de contas, redirecionamentos expõem tokens, e operações com replay, PATCH parcial e formulários podem atingir resultados incorretos.

## What Changes

- Revalidar contas de sessões autenticadas e limitar a capacidade do limiter público de login.
- Corrigir idempotência de PUT/PATCH, identidade de recursos, integridade do replay e coerência entre corpo consumido e fingerprint.
- Validar o término e o limite do JSON e consumir campos multipart sem interferência de query ou colisão entre valores repetidos.
- Preservar campos omitidos em atualizações concorrentes de instâncias e remover JIDs internos de logs/erros de restauração.
- Impedir redirects de clientes que transportam credenciais e cifrar todo token Chatwoot de entrada não vazio.
- Corrigir os fluxos do manager para recado, rename por alias, coordenadas, status sem legenda e revogação de key.
- Isolar recibos por instância e persistir estado terminal de envio junto do evento durável, sem repetir envio durante retry de persistência.
- Preservar chunks novos durante import, manter history-sync inerte sem URI, validar destinatários exatos de contatos e criar sessão no comando init.
- Corrigir a exceção SSRF de anexos Chatwoot para impedir hosts não confiáveis com DNS parcialmente sobreposto.
- Rastrear o placeholder de embed documentado e preservá-lo após build Nuxt para permitir build/test de Go em clone sem assets.

Não há mudança de shape REST, versão de evento, dependência ou migração. As correções aplicam as garantias já estabelecidas e acrescentam cenários explícitos para evitar regressões.

## Capabilities

### New Capabilities

Nenhuma.

### Modified Capabilities

- `wzap-accounts`: sessões vinculadas a contas existentes, roles atuais e limiter de capacidade limitada.
- `wzap-response-contract`: replay íntegro, identificação do recurso e leitura inequívoca dos corpos.
- `wzap-instances`: PATCH parcial concorrente preserva campos omitidos.
- `wzap-operations`: restauração não publica JIDs internos em logs ou erros.
- `wzap-webhooks`: redirects não transportam a credencial de entrega.
- `wzap-chatwoot-config`: entrada opaca sempre cifrada e transporte de token sem redirects.
- `wzap-manager`: formulários e ações continuam corretos com alias e campos opcionais.
- `wzap-message-lifecycle`: recibos isolados por instance_id e wa_id.
- `wzap-inbound-events`: estado terminal e evento durável publicados em conjunto.
- `wzap-chatwoot-import`: reconhecimento somente do lote consumido e coleta inerte sem URI.
- `wzap-chatwoot-inbound`: anexos com exceção privada segura e init no lifecycle de instância fresca.
- `wzap-chatwoot-contacts`: busca e merge apenas entre variantes exatas da mesma identidade.

## Impact

Transportes Go, serviço de instâncias, adapter de sessão, clientes HTTP autenticados, configuração Chatwoot e componentes do manager; testes de regressão e evidências de revisão em conjunto. A entrega será uma alteração local revisável, sem deploy, reinício, commit ou merge automático. Alterações locais preexistentes serão preservadas.

## Out-of-Scope

Novas funcionalidades, redesign visual, mudanças de dependências/workflows/migrações, alteração de logout stateless, mudança da política de cotas, chamadas reais WhatsApp/Chatwoot, teste contra schema compartilhado e uso do stream NATS de produção.
