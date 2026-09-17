# wzap-presence Specification

## Purpose

Permitir simular presença de digitação/pausa e disponibilidade por
instância conectada.

## Requirements

### Requirement: Presença de conversa

O serviço SHALL aceitar presença de conversa (`composing`, `paused`) para
uma conversa e MUST responder `422` para estado desconhecido e `409` quando
desconectada.

#### Scenario: Digitando

- **WHEN** o cliente sinaliza `composing` em conversa válida conectada
- **THEN** a presença é publicada e a resposta confirma o envio

#### Scenario: Estado inválido

- **WHEN** o cliente envia estado fora da allowlist
- **THEN** a resposta é `422` e nada é publicado

### Requirement: Presença de usuário

O serviço SHALL aceitar presença de usuário (`available`, `unavailable`) e
MUST responder `409` quando desconectada.

#### Scenario: Disponível

- **WHEN** o cliente sinaliza `available` em instância conectada
- **THEN** a presença de usuário é publicada e confirmada

### Requirement: Sem presença contínua

O serviço SHALL validar o estado na allowlist antes de tocar a sessão e
MUST NOT oferecer modo contínuo/heartbeat; cada chamada é um sinal pontual.

#### Scenario: Rajada de sinais

- **WHEN** o cliente envia sinais pontuais em sequência
- **THEN** cada um é publicado isoladamente, sem sessão de heartbeat

### Requirement: Autorização da presença

A operação SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Presença sem ownership

- **WHEN** credencial de outro dono sinaliza presença
- **THEN** a resposta é `403` e nada é publicado
