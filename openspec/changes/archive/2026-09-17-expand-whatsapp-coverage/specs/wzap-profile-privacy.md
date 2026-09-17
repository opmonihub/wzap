# wzap-profile-privacy Specification

## Purpose

Permitir consultar e atualizar perfil próprio e ajustes de privacidade por
instância conectada.

## Requirements

### Requirement: Perfil próprio

O serviço SHALL consultar e atualizar nome, recado e foto do perfil, e MUST
responder `409` quando desconectada e `422` para foto inválida ou acima do
limite.

#### Scenario: Nome atualizado

- **WHEN** o cliente atualiza o nome com valor válido
- **THEN** o perfil é atualizado e a resposta confirma

#### Scenario: Foto inválida

- **WHEN** o cliente envia foto em formato não suportado
- **THEN** a resposta é `422` e o perfil permanece intacto

### Requirement: Privacidade

O serviço SHALL consultar e atualizar última visualização, foto de perfil,
status, confirmações de leitura e quem pode adicionar a grupos, e MUST
responder `422` para valor fora do permitido.

#### Scenario: Privacidade atualizada

- **WHEN** o cliente define última visualização como `contacts`
- **THEN** a configuração é aplicada e confirmada

#### Scenario: Valor de privacidade inválido

- **WHEN** o cliente envia valor fora da allowlist
- **THEN** a resposta é `422` e nada muda

### Requirement: Autorização de perfil e privacidade

As operações SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Perfil sem ownership

- **WHEN** credencial de outro dono atualiza o perfil
- **THEN** a resposta é `403` e nada muda
