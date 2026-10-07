## Why

A representação pública da instância foi cortada hoje com `connection` e `webhook` aninhados, mas as configurações que o operador gerencia (webhook, Chatwoot, perfil, privacidade, status privacy e timer de mensagens temporárias) só existem em rotas próprias e não aparecem na leitura da instância. O console precisa de uma leitura única para o detalhe e a listagem, sem penalizar instâncias desconectadas — que hoje responderiam 409 nas rotas vivas.

## What Changes

- Adicionar os blocos `integration` (`webhook` + `chatwoot_config`) e `settings` (`default_disappearing`, `profile`, `privacy`, `status_privacy`) à representação pública da instância, valendo em `GET /instances/{id}` (`data.instance`) e `GET /instances` (`data.items[].instance`), com a mesma representação nas respostas de criação e atualização que devolvem `data.instance`.
- **BREAKING**: `webhook` sai da raiz de `instance` e passa a existir apenas em `integration.webhook` (contrato cortado hoje; consumidor único = Manager Nuxt, atualizado no mesmo corte).
- Semântica de `null` por bloco quando a fonte está indisponível: uma instância desconectada ou uma falha de leitura nunca derruba a resposta nem a listagem.
- Blocos vivos (`profile`, `privacy`, `status_privacy`) só são buscados quando `connection.status == "connected"`.
- `integration.chatwoot_config` é `null` quando não há configuração persistida; quando há, repete a semântica de `GET /instances/{id}/chatwoot` (mesmos campos, sem `instance_id`, token nunca presente).
- `settings.default_disappearing` passa a ecoar o último valor aceito por `PUT /instances/{id}/chats/default-disappearing`, persistido em nova coluna nullable (migração `00009`), com `null` quando nunca configurado — sem valor inventado.
- A listagem agrega os blocos com concorrência limitada por requisição (cerca de 8).
- Privacidade preservada: `external_ref`, `owner_user_id`, JIDs e hashes continuam ocultos; o token Chatwoot continua write-only.
- Swagger, Manager e README atualizados no mesmo corte de contrato.

## Capabilities

### New Capabilities

(nenhuma)

### Modified Capabilities

- `wzap-response-contract`: a representação pública da instância ganha os blocos `integration` e `settings`, com o move **BREAKING** de `webhook` e as garantias de privacidade estendidas aos novos blocos.
- `wzap-instances`: agregação de configurações na leitura, semântica de listagem com `null` por bloco e eco persistido de `default_disappearing`.
- `wzap-manager`: consumo de `integration.webhook` e tolerância a blocos `null` no console.

## Impact

- `internal/httpapi/`: novos DTOs (`integrationResponse`, `settingsResponse`), ajuste de `instanceResponse`, handlers de instância (leitura, listagem, criação, atualização) e reuso do DTO de config Chatwoot.
- `internal/instance/` (e camada de sessão): leituras vivas de perfil/privacidade/status privacy acionadas apenas para instâncias conectadas.
- `internal/storage/` e `internal/storage/postgres/`: migração `00009_default_disappearing.sql` (coluna nullable) e persistência do eco no comando existente de timer.
- `docs/` (Swagger regenerado), `manager/` (tipos e consumo) e `README.md` (matriz REST).
- Consumidores REST do contrato cortado hoje: apenas o Manager, migrado no mesmo corte.

## Out-of-Scope

- Escrita de `integration`/`settings` via `PATCH /instances/{id}` — as rotas próprias (`PUT /instances/{id}/chatwoot`, `PUT /instances/{id}/privacy`, `PATCH /instances/{id}/profile`, `PUT /instances/{id}/chats/default-disappearing`) continuam sendo o caminho de escrita.
- Paginação das listagens (a listagem continua sem página).
- Eventos NATS e entregas de webhook: o contrato de eventos `event_version: 1` é preservado sem alteração.
- Mudanças de RBAC, ownership ou quotas.
- Alterar o comportamento próprio de `GET /instances/{id}/chatwoot`, que hoje sintetiza configuração vazia quando nada está persistido (só o agregado usa `null`).
