## MODIFIED Requirements

### Requirement: Aceite de envio

**BREAKING**: o serviço SHALL aceitar envio em instância connected, validar o conteúdo e enfileirar a mensagem, respondendo 202 com data.message contendo o UUID interno id e send_status inicial queued. Em instância não conectada MUST responder 409 sem enfileirar.

#### Scenario: Envio aceito

- **WHEN** o cliente envia uma mensagem válida para instância conectada
- **THEN** recebe 202 com data.message.id e send_status queued

#### Scenario: Instância não conectada

- **WHEN** o cliente envia mensagem para instância disconnected
- **THEN** recebe 409 e nada é enfileirado

### Requirement: Idempotência de envio

Requisições de envio SHALL aceitar Idempotency-Key. Mesmo conteúdo SHALL recuperar o resultado anterior marcado como replay; chave com conteúdo diferente MUST responder 422; concorrência com a mesma chave MUST responder 409. Sem chave o comportamento é normal. Respostas antigas SHALL ser convertidas para o novo contrato mantendo o UUID sem reexecutar o envio.

#### Scenario: Replay idempotente

- **WHEN** o cliente repete o envio com a mesma chave e conteúdo
- **THEN** recupera o mesmo UUID em data.message com indicador de replay sem nova mensagem

#### Scenario: Conflito de conteúdo

- **WHEN** o cliente repete a chave com conteúdo diferente
- **THEN** recebe 422 e a mensagem original permanece intacta

#### Scenario: Replay de contrato legado

- **WHEN** o cache contém o antigo message_id de uma operação concluída
- **THEN** a resposta atual conserva esse UUID como id sem executar o envio outra vez

## ADDED Requirements

### Requirement: Representação pública de envio

Respostas de mensagens SHALL distinguir id interno e wa_id externo, usar message_type, recipient_jid, send_status e retry_count, e apresentar last_error estruturado ou null. Datas de entrega e leitura SHALL conservar seu significado.

#### Scenario: Envio confirmado

- **WHEN** o WhatsApp confirma um envio
- **THEN** a consulta identifica o mesmo UUID interno e o wa_id retornado, conservando datas e escopo da instância
