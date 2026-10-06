## ADDED Requirements

### Requirement: Consulta do perfil próprio conectado

`GET /instances/{id}/profile` de instância conectada SHALL responder `200` com o push name conhecido pela sessão. O identificador próprio armazenado com sufixo de dispositivo MUST NOT fazer a consulta falhar. Recado e URL da foto SHALL vir preenchidos quando o upstream os informar e vazios quando não houver; essa ausência MUST NOT produzir `500`. Instância desconectada continua `409`.

#### Scenario: JID próprio com dispositivo

- **WHEN** a instância está conectada e o identificador armazenado inclui sufixo de dispositivo
- **THEN** a resposta é `200` e traz o push name

#### Scenario: Upstream sem recado ou sem foto

- **WHEN** a instância está conectada e o upstream não informa recado ou foto
- **THEN** a resposta é `200`, o push name vem preenchido, e recado ou URL da foto vêm vazios
