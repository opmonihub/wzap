# wzap-group-moderation Specification

## Purpose
TBD - created by archiving change whatsmeow-parity-routes. Update Purpose after archive.

## Requirements

### Requirement: Lista de grupos da instância

O serviço SHALL listar os grupos em que a instância está, com paginação por
cursor (`limit` padrão 50, máximo 100, `next_cursor` com o último grupo),
expondo identificador, nome e contagem de participantes, e MUST responder
`409` quando desconectada.

#### Scenario: Lista paginada

- **WHEN** o cliente lista grupos com limite válido em instância conectada
- **THEN** recebe uma página com itens e o próximo cursor

### Requirement: Prévia de convite

O serviço SHALL mostrar a prévia do grupo de um código/convite sem entrar
(nome, descrição, participantes visíveis) e MUST responder `422` para
convite inválido ou expirado.

#### Scenario: Prévia válida

- **WHEN** o cliente informa convite válido
- **THEN** a prévia é devolvida e a instância não entra no grupo

#### Scenario: Convite inválido

- **WHEN** o cliente informa convite desconhecido ou expirado
- **THEN** a resposta é `422` e nada muda

### Requirement: Pedidos de entrada

O serviço SHALL listar pedidos pendentes do grupo e aprovar/recusar
(`approve`|`decline` + `participants[]`), e MUST responder `403` quando o
solicitante não tem permissão no grupo e `404` para grupo desconhecido.

#### Scenario: Pedido aprovado

- **WHEN** admin aprova pedido pendente válido
- **THEN** o participante entra e a resposta confirma

#### Scenario: Sem permissão no grupo

- **WHEN** não-admin tenta aprovar pedido
- **THEN** a resposta é `403` e nada muda

### Requirement: Travas do grupo

O serviço SHALL ler e atualizar travas (`announce`, `locked`,
`join_approval`, `member_add_mode` dentro da allowlist) e MUST responder
`422` para valor fora do permitido e `403` sem permissão no grupo.

#### Scenario: Grupo anunciado

- **WHEN** admin ativa modo somente-admin (`announce=true`)
- **THEN** a trava é aplicada e a resposta confirma

#### Scenario: Valor inválido

- **WHEN** o cliente envia modo de adição fora da allowlist
- **THEN** a resposta é `422` e nada muda

### Requirement: Idempotência e autorização da moderação

Escritas SHALL aceitar `Idempotency-Key` (replay, `422` divergente, `409`
concorrência). Todas as operações SHALL exigir dual auth e ownership e
MUST responder `404` em instância inexistente e `403` sem ownership.

#### Scenario: Replay de aprovação

- **WHEN** o cliente repete a key com o mesmo lote de aprovação
- **THEN** a resposta original é devolvida com indicador de replay
