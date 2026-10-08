## MODIFIED Requirements

### Requirement: Representação pública de envio

**BREAKING**: respostas de mensagens SHALL distinguir id interno e wa_id externo, usar message_type, recipient_jid, send_status e retry_count, e apresentar `last_error` estruturado somente quando houver erro. Campos opcionais `wa_id`, `media_id`, `next_attempt_at`, `delivered_at`, `read_at` e `last_error` SHALL ser omitidos quando ausentes; `occurred_at` SHALL ser omitido quando desconhecido. `retry_count` MUST permanecer presente inclusive quando zero. Listagem e detalhe SHALL usar a mesma representação para o mesmo estado e contexto. Datas de entrega e leitura SHALL conservar seu significado.

#### Scenario: Envio confirmado

- **WHEN** o WhatsApp confirma um envio
- **THEN** a consulta identifica o mesmo UUID interno e o wa_id retornado, conservando datas e escopo da instância

#### Scenario: Mensagem sem confirmação nem falha

- **WHEN** uma mensagem ainda não possui confirmação, mídia, erro ou marcos opcionais de entrega, leitura e próxima tentativa
- **THEN** os opcionais ausentes são omitidos e os campos obrigatórios, incluindo `retry_count` igual a `0`, permanecem presentes

#### Scenario: Listagem de mensagens

- **WHEN** uma instância possui mensagens no escopo autorizado
- **THEN** `data.messages` contém diretamente as representações públicas equivalentes ao detalhe, sem wrappers `message`

### Requirement: Aceite de envio

**BREAKING**: o serviço SHALL aceitar envio em instância connected, validar o conteúdo e enfileirar a mensagem, respondendo 202 com data.message contendo o UUID interno id e send_status inicial queued. Em instância não conectada MUST responder 409 sem enfileirar.

#### Scenario: Envio aceito

- **WHEN** o cliente envia uma mensagem válida para instância conectada
- **THEN** recebe 202 com data.message.id e send_status queued

#### Scenario: Instância não conectada

- **WHEN** o cliente envia mensagem para instância disconnected
- **THEN** recebe 409 e nada é enfileirado

#### Scenario: Aceite sem mídia

- **WHEN** um envio aceito não possui mídia
- **THEN** `data.message` conserva a representação resumida obrigatória e omite `media_id`

#### Scenario: Aceite com mídia

- **WHEN** um envio aceito possui mídia persistida conhecida
- **THEN** `data.message.media_id` contém o identificador da mídia
