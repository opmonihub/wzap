# wzap-outbound-messaging Specification

## Purpose

Permitir que o backend envie mensagens por uma instância conectada de forma assíncrona, idempotente e observável.

## Requirements

### Requirement: Aceite de envio

**BREAKING**: o serviço SHALL aceitar envio em instância connected, validar o conteúdo e enfileirar a mensagem, respondendo 202 com data.message contendo o UUID interno id e send_status inicial queued. Em instância não conectada MUST responder 409 sem enfileirar.

#### Scenario: Envio aceito

- **WHEN** o cliente envia uma mensagem válida para instância conectada
- **THEN** recebe 202 com data.message.id e send_status queued

#### Scenario: Instância não conectada

- **WHEN** o cliente envia mensagem para instância disconnected
- **THEN** recebe 409 e nada é enfileirado

### Requirement: Normalização de destinatário

O serviço SHALL normalizar o destinatário para o identificador canônico do WhatsApp, aplicando a regra brasileira do 9º dígito, e MUST responder `422` quando o número não existir na plataforma.

#### Scenario: Número válido com variação brasileira

- **WHEN** o cliente informa um número brasileiro válido com ou sem o 9º dígito
- **THEN** o serviço resolve o identificador correto e enfileira a mensagem

#### Scenario: Número inexistente

- **WHEN** o cliente informa número que não existe no WhatsApp
- **THEN** a resposta é `422` e nada é enfileirado

### Requirement: Tipos de mensagem suportados

O serviço SHALL suportar texto, imagem, vídeo, áudio (incluindo mensagem de voz), documento, localização e contato; conteúdo inválido ou tipo não suportado MUST responder `422`. Tipos ricos (enquete, reação, figurinha via `type=sticker` no upload, lista, botões) seguem `wzap-rich-messaging` no mesmo aceite `202` idempotente.

#### Scenario: Mídia inválida

- **WHEN** o cliente envia arquivo de tipo não permitido ou acima do limite
- **THEN** a resposta é `422` e nada é enfileirado

### Requirement: Ciclo de entrega

Cada mensagem enfileirada MUST transitar por estados observáveis até `sent` ou `failed`. Falhas transitórias SHALL ser retentadas com espera crescente até um limite; falhas definitivas MUST registrar o motivo.

#### Scenario: Envio bem-sucedido

- **WHEN** a mensagem é entregue ao protocolo
- **THEN** o estado final é `sent` com identificador do WhatsApp registrado

#### Scenario: Falha definitiva

- **WHEN** a mensagem falha por motivo não recuperável após o limite de tentativas
- **THEN** o estado final é `failed` com o motivo registrado

### Requirement: Consulta de status

O serviço SHALL permitir consultar uma mensagem enviada, retornando estado atual, marcos temporais e último erro quando houver.

#### Scenario: Consulta de mensagem existente

- **WHEN** o cliente consulta o identificador de uma mensagem enviada
- **THEN** recebe estado, timestamps e erro (se houver)

### Requirement: Recibos de entrega e leitura

Recibos recebidos SHALL atualizar o registro da mensagem correspondente, tornando `delivered` e `read` observáveis.

#### Scenario: Mensagem lida

- **WHEN** chega recibo de leitura da mensagem enviada
- **THEN** o registro passa a indicar leitura com o momento correspondente

### Requirement: Recuperação após interrupção

Mensagens presas em processamento além de um limite SHALL ser reenfileiradas ou marcadas como falhas na recuperação, e a possibilidade de reenvio duplicado MUST ser observável.

#### Scenario: Reinício durante envio

- **WHEN** o serviço reinicia com mensagem em processamento
- **THEN** a mensagem é retomada ou marcada como falha, sem ficar indefinidamente presa

### Requirement: Humanização opcional

Quando habilitada, o serviço SHALL simular presença antes do envio; quando desabilitada, MUST NOT introduzir atraso além do necessário ao protocolo.

#### Scenario: Humanização habilitada

- **WHEN** a configuração de humanização está ativa e uma mensagem é processada
- **THEN** a presença é simulada antes do envio

### Requirement: Representação pública de envio

Respostas de mensagens SHALL distinguir id interno e wa_id externo, usar message_type, recipient_jid, send_status e retry_count, e apresentar last_error estruturado ou null. Datas de entrega e leitura SHALL conservar seu significado.

#### Scenario: Envio confirmado

- **WHEN** o WhatsApp confirma um envio
- **THEN** a consulta identifica o mesmo UUID interno e o wa_id retornado, conservando datas e escopo da instância

### Requirement: Idempotência do contrato vigente

Requisições de envio SHALL aceitar Idempotency-Key. Mesmo conteúdo SHALL recuperar o resultado anterior marcado como replay; chave com conteúdo diferente MUST responder 422; concorrência com a mesma chave MUST responder 409. Sem chave o comportamento é normal. O replay SHALL devolver o status HTTP e o corpo armazenados no contrato atual sem conversão, reenvio ou criação de nova mensagem.

#### Scenario: Replay idempotente

- **WHEN** o cliente repete o envio com a mesma chave e conteúdo
- **THEN** recupera o mesmo UUID em data.message com indicador de replay sem nova mensagem

#### Scenario: Conflito de conteúdo

- **WHEN** o cliente repete a chave com conteúdo diferente
- **THEN** recebe 422 e a mensagem original permanece intacta

#### Scenario: Chave em processamento

- **WHEN** dois envios concorrentes disputam a mesma chave e conteúdo
- **THEN** a requisição concorrente recebe 409 e apenas a operação que adquiriu a chave pode produzir o envio
