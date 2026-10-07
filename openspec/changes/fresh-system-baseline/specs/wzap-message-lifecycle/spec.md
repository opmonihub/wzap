## MODIFIED Requirements

### Requirement: Formato de resposta da revogação

**BREAKING:** o adapter não sinaliza "fora da janela" (só erro de conexão, transitório
ou destinatário inválido): envio bem-sucedido ao protocolo SHALL responder
`200` com `revoked: true`; alvo inválido SHALL responder `422`, nunca
`500`. A resposta de sucesso SHALL conter somente o resultado atual revoked no envelope data, sem campo reservado para compatibilidade futura.

#### Scenario: Envio aceito pelo protocolo

- **WHEN** o protocolo aceita o REVOKE
- **THEN** a resposta é `200` com `revoked: true` e sem `reason`
