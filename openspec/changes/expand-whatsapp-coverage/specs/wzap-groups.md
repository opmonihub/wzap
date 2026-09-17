# wzap-groups Specification

## Purpose

Permitir gerenciar grupos (ciclo de vida, membros e convites) por
instância conectada.

## Requirements

### Requirement: Ciclo de vida do grupo

O serviço SHALL criar, consultar e atualizar grupo (assunto, descrição,
foto) e permitir entrar/sair, e MUST responder `409` quando desconectada e
`422` para dados inválidos.

#### Scenario: Grupo criado

- **WHEN** o cliente cria grupo válido com participantes em instância
  conectada
- **THEN** o grupo é criado e a resposta traz o identificador e o convite
  quando aplicável

#### Scenario: Grupo inexistente

- **WHEN** o cliente consulta grupo desconhecido
- **THEN** a resposta é `404`

### Requirement: Membros e admins

O serviço SHALL adicionar/remover participantes e promover/rebaixar admins,
e MUST responder `403` quando o solicitante não tem permissão no grupo.

#### Scenario: Participante removido

- **WHEN** admin remove participante válido
- **THEN** a remoção é aplicada e confirmada

#### Scenario: Sem permissão no grupo

- **WHEN** não-admin tenta promover participante
- **THEN** a resposta é `403` e nada muda

### Requirement: Convites

O serviço SHALL gerar, consultar e revogar código de convite e permitir
entrar por convite válido, e MUST responder `422` para convite inválido.

#### Scenario: Entrada por convite

- **WHEN** o cliente entra com convite válido
- **THEN** a instância passa a integrar o grupo e a resposta confirma

### Requirement: Eventos de grupo

Mudanças de grupo (membro entrou/saiu, assunto alterado) SHALL gerar eventos
versionados com `event_id` estável.

#### Scenario: Membro entrou

- **WHEN** um membro entra no grupo da instância
- **THEN** um evento de grupo é publicado com o ator e o afetado
