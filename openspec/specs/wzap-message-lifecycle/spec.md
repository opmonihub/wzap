# wzap-message-lifecycle Specification

## Purpose

Permitir revogar mensagens enviadas e confirmar leitura de mensagens
recebidas por instância conectada.

## Requirements

### Requirement: Revogação de mensagem enviada

O serviço SHALL revogar para todos uma mensagem enviada pela instância e
MUST responder `409` quando desconectada e `422` quando o alvo for inválido
ou a janela tiver expirado.

#### Scenario: Revogação dentro da janela

- **WHEN** o cliente revoga mensagem enviada válida em instância conectada
- **THEN** a mensagem é revogada para todos e a resposta confirma `revoked:
  true`

#### Scenario: Instância desconectada

- **WHEN** o cliente revoga mensagem com instância desconectada
- **THEN** a resposta é `409` e nada é revogado

#### Scenario: Alvo inválido ou fora da janela

- **WHEN** o cliente revoga mensagem inexistente ou fora da janela do
  protocolo
- **THEN** a resposta é `422` (ou `200` com `revoked: false` documentado) e
  o motivo é informado sem detalhe interno

### Requirement: Formato de resposta da revogação

**BREAKING:** o adapter não sinaliza "fora da janela" (só erro de conexão, transitório
ou destinatário inválido): envio bem-sucedido ao protocolo SHALL responder
`200` com `revoked: true`; alvo inválido SHALL responder `422`, nunca
`500`. A resposta de sucesso SHALL conter somente o resultado atual revoked no envelope data, sem campo reservado para compatibilidade futura.

#### Scenario: Envio aceito pelo protocolo

- **WHEN** o protocolo aceita o REVOKE
- **THEN** a resposta é `200` com `revoked: true` e sem `reason`

### Requirement: Confirmação de leitura

O serviço SHALL enviar recibo de leitura de mensagem em conversa, exigindo o
autor em grupos e completando-o em conversas diretas, e MUST responder `409`
quando desconectada.

#### Scenario: Leitura em conversa direta sem autor

- **WHEN** o cliente confirma leitura informando só conversa e mensagem
- **THEN** o recibo é enviado e a resposta confirma o envio

#### Scenario: Leitura em grupo sem autor

- **WHEN** o cliente confirma leitura em grupo sem informar o autor
- **THEN** a resposta é `422` e nenhum recibo é enviado

### Requirement: Autorização e erros do ciclo de vida

As operações SHALL exigir a mesma dual auth e ownership das mensagens e
MUST responder `404` em instância inexistente e `403` sem ownership.

#### Scenario: Revogação sem ownership

- **WHEN** credencial válida de outro dono tenta revogar mensagem
- **THEN** a resposta é `403` e nada é revogado
