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

### Requirement: Limites e suporte do perfil

Nome SHALL ter 1..100 caracteres; recado 0..500 (vazio limpa); foto
`image/*` via pipeline com cap de `WZAP_MAX_MEDIA_BYTES` (acima → `413`).
Nome e foto SHALL responder `501 not_supported` (a lib pinada não expõe
os setters); quando `name` vem no patch, nada é aplicado — nem o recado.
Recado e consulta de perfil (push name best-effort) são suportados de
verdade.

#### Scenario: Nome com recado junto

- **WHEN** o cliente envia patch com `name` e `status_text` válidos
- **THEN** a resposta é `501` e nada muda, incluindo o recado

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

### Requirement: Allowlists de privacidade

`last_seen`, `profile_photo`, `status` e `groups_add` SHALL aceitar
`all`|`contacts`|`contact_blacklist`|`none`; `read_receipts` SHALL aceitar
`all`|`none` (a lib modela receipts como switch de dois valores, não
boolean). Todas as operações de privacidade são suportadas (sem `501`);
`SetPrivacy` não é atômico — falha parcial pode aplicar um subset.

#### Scenario: Recibos como none

- **WHEN** o cliente define `read_receipts` como `none`
- **THEN** a configuração é aplicada e confirmada

### Requirement: Autorização de perfil e privacidade

As operações SHALL exigir dual auth e ownership e MUST responder `404` em
instância inexistente e `403` sem ownership.

#### Scenario: Perfil sem ownership

- **WHEN** credencial de outro dono atualiza o perfil
- **THEN** a resposta é `403` e nada muda

### Requirement: Consulta do perfil próprio conectado

`GET /instances/{id}/profile` de instância conectada SHALL responder `200` com o push name conhecido pela sessão. O identificador próprio armazenado com sufixo de dispositivo MUST NOT fazer a consulta falhar. Recado e URL da foto SHALL vir preenchidos quando o upstream os informar e vazios quando não houver; essa ausência MUST NOT produzir `500`. Instância desconectada continua `409`.

#### Scenario: JID próprio com dispositivo

- **WHEN** a instância está conectada e o identificador armazenado inclui sufixo de dispositivo
- **THEN** a resposta é `200` e traz o push name

#### Scenario: Upstream sem recado ou sem foto

- **WHEN** a instância está conectada e o upstream não informa recado ou foto
- **THEN** a resposta é `200`, o push name vem preenchido, e recado ou URL da foto vêm vazios
