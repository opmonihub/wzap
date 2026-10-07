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

### Requirement: Limites por tipo

Enquete SHALL ter pergunta de 1..300 caracteres, 2..12 opções de 1..100
caracteres cada e `selectable_count` 0 ou 1 (omitido vira 1). Lista SHALL
ter 1..10 seções de 1..10 linhas cada e textos de 1..300 caracteres.
Botões SHALL ter 1..3 botões com `id`/`title` de até 64 caracteres e corpo
e rodapé de até 300. Fora dos limites, a resposta MUST ser `422` e nada
SHALL ser persistido.

#### Scenario: Enquete acima do limite

- **WHEN** o cliente envia enquete com 13 opções
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

### Requirement: Reação de saída

Reação de saída com emoji vazio SHALL remover a reação anterior no mesmo
endpoint; reações endereçam mensagens enviadas pela própria instância.

#### Scenario: Reação removida

- **WHEN** o cliente envia reação com emoji vazio para mensagem própria
- **THEN** a remoção é enfileirada com `202`

### Requirement: Sem 501 por tipo não suportado

Todo tipo rico é construtível no upstream pinado; o serviço MUST NOT
emitir `501` por tipo não suportado neste aceite (suporte documentado por
operação no Swagger).

#### Scenario: Tipo rico documentado

- **WHEN** o cliente envia um tipo rico documentado no Swagger para instância conectada
- **THEN** o aceite não responde `501` para o tipo

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

### Requirement: Nomes de evento e subjects

Os tipos SHALL ser `poll.vote`, `message.reaction` e
`interactive.response`, nos subjects
`wzap.instances.<id>.message.poll.vote`,
`wzap.instances.<id>.message.reaction` e
`wzap.instances.<id>.message.interactive.response`. Voto
não-descriptografável (sem secrets) SHALL ser emitido mesmo assim com
eleitor + chave da enquete e `selected_option_names` vazio (o fio carrega
só hashes irreversíveis).

#### Scenario: Voto sem secrets

- **WHEN** chega voto cuja descriptografia falha
- **THEN** o evento é publicado com eleitor e chave, sem opções

### Requirement: Webhook opt-in

Os três tipos SHALL validar em `webhook_events` explícito mas MUST NOT
entrar no default: omitir `webhook_events` assina exatamente os 4 tipos
originais.

#### Scenario: Default pinado

- **WHEN** o operador omite `webhook_events`
- **THEN** votos, reações e respostas não são entregues via webhook
