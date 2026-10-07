## ADDED Requirements

### Requirement: Consulta da lista de bloqueados

O serviço SHALL devolver a lista de JIDs bloqueados pela instância e MUST
responder `409` quando desconectada.

#### Scenario: Lista consultada

- **WHEN** o cliente consulta a blocklist em instância conectada
- **THEN** recebe os JIDs bloqueados

### Requirement: Bloquear e desbloquear

O serviço SHALL bloquear (`block`) e desbloquear (`unblock`) um JID válido;
bloquear JID já bloqueado (ou desbloquear não bloqueado) SHALL ser
idempotente com resposta de confirmação, e MUST responder `422` para ação
ou JID inválido.

#### Scenario: Contato bloqueado

- **WHEN** o cliente bloqueia JID válido em instância conectada
- **THEN** o contato entra na blocklist e a resposta confirma

#### Scenario: Ação inválida

- **WHEN** o cliente envia ação fora de `block`|`unblock`
- **THEN** a resposta é `422` e nada muda

### Requirement: Idempotência e autorização do bloqueio

Escritas SHALL aceitar `Idempotency-Key` (replay, `422` divergente, `409`
concorrência). As operações SHALL exigir dual auth e ownership e MUST
responder `404` em instância inexistente e `403` sem ownership.

#### Scenario: Replay de bloqueio

- **WHEN** o cliente repete a key com o mesmo `action`+`jid`
- **THEN** a resposta original é devolvida com indicador de replay
