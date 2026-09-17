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

### Requirement: Nome do grupo em runas

O assunto do grupo SHALL ser limitado a 25 runas (limite do upstream, não
bytes); acima disso a resposta MUST ser `422`.

#### Scenario: Nome multibyte acima do limite

- **WHEN** o cliente cria grupo com nome de 26 runas multibyte
- **THEN** a resposta é `422` e nada é criado

### Requirement: Criação parcial com convite

`POST /groups` lê o convite após criar: se a leitura falhar, o grupo já
existe e a resposta SHALL continuar `201` com o grupo e `invite_code`
vazio (`omitempty`); o cliente MUST NOT repetir o create e SHALL
reconciliar via `GET .../invite`.

#### Scenario: Convite pós-create falha

- **WHEN** o grupo é criado mas a leitura do convite falha
- **THEN** a resposta é `201` com o identificador e `invite_code` vazio

### Requirement: Membros e admins

O serviço SHALL adicionar/remover participantes e promover/rebaixar admins,
e MUST responder `403` quando o solicitante não tem permissão no grupo.

#### Scenario: Participante removido

- **WHEN** admin remove participante válido
- **THEN** a remoção é aplicada e confirmada

#### Scenario: Sem permissão no grupo

- **WHEN** não-admin tenta promover participante
- **THEN** a resposta é `403` e nada muda

### Requirement: Ação de participantes em allowlist

A ação SHALL ser `add`, `remove`, `promote` ou `demote`; fora disso a
resposta MUST ser `422`. Saída própria é pela rota de saída; foto de grupo
SHALL aceitar corpo `image/*` cru com cap de `WZAP_MAX_MEDIA_BYTES`
(acima → `413`) e só troca (sem remoção).

#### Scenario: Ação desconhecida

- **WHEN** o cliente envia ação fora da allowlist
- **THEN** a resposta é `422` e nada muda

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

### Requirement: Nomes de evento de grupo

Os tipos SHALL ser `group.participants` (adesões/saídas/remoções) e
`group.info` (assunto/tópico/foto), nos subjects
`wzap.instances.<id>.group.participants` e
`wzap.instances.<id>.group.info`, fora de `message.*`. Ambos SHALL ser
opt-in de webhook com o default pinado nos 4 originais.

#### Scenario: Assunto alterado

- **WHEN** o assunto do grupo muda
- **THEN** um evento `group.info` é publicado com o snapshot de nome/tópico

### Requirement: Metadados com refresh sob demanda

Toda leitura live (criar/consultar/atualizar grupo, seguir/consultar/listar
canal) SHALL fazer write-through do cache de metadados e expor
`updated_at`; o cache é log de refresh, nunca fonte (sem leitura
stale/TTL). Falha de cache SHALL só logar e devolver o live; sem store, a
leitura passa direto com `updated_at` zero.

#### Scenario: Leitura reflete o upstream

- **WHEN** o cliente consulta grupo após mudança externa
- **THEN** a resposta traz o estado atual do upstream com `updated_at`
