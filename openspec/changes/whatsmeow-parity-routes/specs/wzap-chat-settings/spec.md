# wzap-chat-settings Specification

## Purpose

Permitir ajustar conversas: temporizador de desaparecimento, privacidade de
status (leitura) e assinatura pontual de presença, por instância conectada.

## Requirements

### Requirement: Temporizador por conversa e padrão

O serviço SHALL definir o temporizador de mensagens que desaparecem por
conversa (`duration`: `0` desliga, ou 24h/7d/90d) e o padrão de novas
conversas, além de consultar o valor vigente, e MUST responder `422` para
duração fora da allowlist e `409` quando desconectada.

#### Scenario: Temporizador ativado

- **WHEN** o cliente define 24h em conversa válida conectada
- **THEN** o temporizador é aplicado e a resposta confirma

#### Scenario: Duração inválida

- **WHEN** o cliente envia duração fora da allowlist
- **THEN** a resposta é `422` e nada muda

### Requirement: Privacidade de status (leitura)

O serviço SHALL consultar a audiência do status (lista de visibilidade do
upstream) e MUST responder `409` quando desconectada.

#### Scenario: Audiência consultada

- **WHEN** o cliente consulta a privacidade do status conectado
- **THEN** recebe a audiência vigente

### Requirement: Assinatura pontual de presença

O serviço SHALL assinar a presença de um contato (sinal pontual, sem
heartbeat/contínuo) e MUST responder `422` para JID inválido e `409`
quando desconectada.

#### Scenario: Presença assinada

- **WHEN** o cliente assina contato válido em instância conectada
- **THEN** a assinatura é registrada e a resposta confirma

### Requirement: Idempotência e autorização dos ajustes

Escritas SHALL aceitar `Idempotency-Key` (replay, `422` divergente, `409`
concorrência). As operações SHALL exigir dual auth e ownership e MUST
responder `404` em instância inexistente e `403` sem ownership.

#### Scenario: Ajuste sem ownership

- **WHEN** credencial de outro dono define temporizador
- **THEN** a resposta é `403` e nada muda
