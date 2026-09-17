# wzap-status-calls Specification

## Purpose

Permitir publicar e consultar status/stories e observar (e rejeitar quando
possível) chamadas por instância conectada.

## Requirements

### Requirement: Publicação de status

O serviço SHALL publicar status de texto, imagem e vídeo com expiração do
protocolo, e MUST responder `409` quando desconectada e `422` para mídia
inválida ou acima do limite.

#### Scenario: Status publicado

- **WHEN** o cliente publica texto válido em instância conectada
- **THEN** o status é publicado e a resposta traz o identificador

#### Scenario: Mídia de status inválida

- **WHEN** o cliente publica arquivo acima do limite
- **THEN** a resposta é `422` e nada é publicado

### Requirement: Backing síncrono com 202

Publicar status SHALL responder `202` + `message_id` com replay
`X-Idempotent-Replay` via o middleware de idempotência, mas o backing é
síncrono fire-and-forget para o broadcast — sem retry de outbox. Texto e
caption SHALL ter 1..700 runas.

#### Scenario: Replay de publicação

- **WHEN** o cliente repete a publicação com a mesma `Idempotency-Key` e o
  mesmo conteúdo
- **THEN** o `message_id` original é devolvido com indicador de replay e
  uma só publicação ocorre

### Requirement: Consulta e remoção de status

O serviço SHALL listar os status vigentes publicados desde o boot e apagar o próprio status (a expiração do protocolo de ~24h aplica-se), e MUST
responder `404` para status desconhecido.

#### Scenario: Status apagado

- **WHEN** o cliente apaga o próprio status vigente
- **THEN** o status é removido e a resposta confirma

### Requirement: Poda de expirados

Entradas expiram no upstream após ~24h; o registry local SHALL descartá-las
na listagem.

#### Scenario: Listagem após 24h

- **WHEN** o cliente lista status com entradas expiradas
- **THEN** as expiradas não aparecem

### Requirement: Observação de chamadas

O serviço SHALL publicar eventos versionados de chamada (oferta, aceite,
recusa, fim) com `event_id` estável e MUST NOT prometer iniciar chamada
pelo companion.

#### Scenario: Chamada ofertada

- **WHEN** chega oferta de chamada para a instância
- **THEN** um evento de chamada é publicado com o autor e o tipo

### Requirement: Evento único de chamada

O tipo SHALL ser o unificado `call.offer` com campo `state`
(`offer`|`accept`|`reject`|`end`), no subject
`wzap.instances.<id>.call.offer`, opt-in de webhook com o default pinado
nos 4 originais. O companion MUST NOT iniciar chamada.

#### Scenario: Chamada aceita em outro device

- **WHEN** chega aceite de chamada da instância
- **THEN** um evento `call.offer` é publicado com `state: accept`

### Requirement: Rejeição de chamada

O serviço SHALL rejeitar a chamada ativa quando o protocolo permitir e MUST
responder `501` documentado quando não suportado.

#### Scenario: Chamada rejeitada

- **WHEN** o cliente rejeita chamada ativa suportada
- **THEN** a chamada é rejeitada e a resposta confirma

#### Scenario: Rejeição não suportada

- **WHEN** o cliente rejeita chamada onde o protocolo não permite
- **THEN** a resposta é `501` com código estável
