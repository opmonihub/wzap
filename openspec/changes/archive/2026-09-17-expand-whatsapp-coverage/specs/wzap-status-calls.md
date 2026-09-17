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

### Requirement: Consulta e remoção de status

O serviço SHALL listar os status vigentes e apagar o próprio status, e MUST
responder `404` para status desconhecido.

#### Scenario: Status apagado

- **WHEN** o cliente apaga o próprio status vigente
- **THEN** o status é removido e a resposta confirma

### Requirement: Observação de chamadas

O serviço SHALL publicar eventos versionados de chamada (oferta, aceite,
recusa, fim) com `event_id` estável e MUST NOT prometer iniciar chamada
pelo companion.

#### Scenario: Chamada ofertada

- **WHEN** chega oferta de chamada para a instância
- **THEN** um evento de chamada é publicado com o autor e o tipo

### Requirement: Rejeição de chamada

O serviço SHALL rejeitar a chamada ativa quando o protocolo permitir e MUST
responder `501` documentado quando não suportado.

#### Scenario: Chamada rejeitada

- **WHEN** o cliente rejeita chamada ativa suportada
- **THEN** a chamada é rejeitada e a resposta confirma

#### Scenario: Rejeição não suportada

- **WHEN** o cliente rejeita chamada onde o protocolo não permite
- **THEN** a resposta é `501` com código estável
