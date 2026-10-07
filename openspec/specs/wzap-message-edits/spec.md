# wzap-message-edits Specification

## Purpose

Editar mensagens enviadas no WhatsApp quando a instância está conectada e a mensagem ainda é editável.

## Requirements

### Requirement: Edição de texto próprio

O serviço SHALL aceitar edição de mensagem de texto enviada pela própria
instância (`chat`, `message_id`, `text` de 1..4096 caracteres) e MUST
responder `409` quando desconectada, `422` para texto inválido ou alvo que
não aceita edição (mídia, enquete, mensagem de terceiros) e `404` para
mensagem desconhecida no upstream.

#### Scenario: Texto editado

- **WHEN** o cliente edita texto próprio válido em instância conectada
- **THEN** a edição é aplicada e a resposta confirma com o identificador

#### Scenario: Alvo sem edição

- **WHEN** o cliente tenta editar mídia ou mensagem de terceiros
- **THEN** a resposta é `422` e nada muda

### Requirement: Idempotência da edição

Requisições SHALL aceitar `Idempotency-Key` com a mesma semântica do envio:
replay devolve o original, conteúdo divergente responde `422` e
concorrência responde `409`.

#### Scenario: Replay de edição

- **WHEN** o cliente repete a key com o mesmo `chat`/`message_id`/`text`
- **THEN** a resposta original é devolvida com indicador de replay

### Requirement: Autorização da edição

A operação SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Edição sem ownership

- **WHEN** credencial de outro dono edita mensagem
- **THEN** a resposta é `403` e nada muda
