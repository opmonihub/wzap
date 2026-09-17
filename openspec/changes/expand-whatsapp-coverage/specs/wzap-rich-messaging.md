# wzap-rich-messaging Specification

## Purpose

Permitir enviar e receber enquetes, reações, figurinhas, listas e botões
interativos pelo mesmo aceite assíncrono e idempotente das mensagens.

## Requirements

### Requirement: Aceite de mensagem rica

O serviço SHALL aceitar enquete, reação, figurinha, lista e botões em
instância conectada com `202` + identificador, e MUST responder `409`
quando desconectada e `422` para conteúdo inválido.

#### Scenario: Enquete aceita

- **WHEN** o cliente envia enquete válida (pergunta + ≥2 opções) conectada
- **THEN** a resposta é `202` com `message_id` e estado `queued`

#### Scenario: Conteúdo inválido

- **WHEN** o cliente envia enquete sem opções ou reação sem alvo
- **THEN** a resposta é `422` e nada é enfileirado

### Requirement: Idempotência de mensagem rica

Requisições SHALL aceitar `Idempotency-Key` com a mesma semântica do texto:
replay devolve o original, conteúdo divergente responde `422` e
concorrência responde `409`.

#### Scenario: Replay de enquete

- **WHEN** o cliente repete a key com o mesmo conteúdo
- **THEN** o `message_id` original é devolvido com indicador de replay

### Requirement: Figurinha via upload existente

O serviço SHALL aceitar figurinha pelo upload multipart com `type=sticker`,
validando webp e o limite de tamanho, e MUST responder `422` para tipo ou
tamanho inválido.

#### Scenario: Figurinha válida

- **WHEN** o cliente envia webp válido dentro do limite
- **THEN** a resposta é `202` com `message_id`

### Requirement: Entrada de mensagem rica

Votos de enquete, reações recebidas e respostas de lista/botão SHALL gerar
eventos versionados com `event_id` estável, entregues via NATS e webhook
quando assinados.

#### Scenario: Voto recebido

- **WHEN** chega voto em enquete da instância
- **THEN** um evento de voto é publicado com o autor e a opção escolhida

#### Scenario: Reação recebida

- **WHEN** chega reação a mensagem da instância
- **THEN** um evento de reação é publicado com o emoji e o alvo
