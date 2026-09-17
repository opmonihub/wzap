# wzap-newsletters Specification

## Purpose

Permitir seguir, consultar e listar newsletters (canais) por instância
conectada.

## Requirements

### Requirement: Seguir e deixar de seguir

O serviço SHALL seguir e deixar de seguir canal válido, e MUST responder
`409` quando desconectada e `404` para canal desconhecido.

#### Scenario: Canal seguido

- **WHEN** o cliente segue canal existente em instância conectada
- **THEN** a assinatura é criada e a resposta confirma

#### Scenario: Canal desconhecido

- **WHEN** o cliente segue canal inexistente
- **THEN** a resposta é `404` e nada é assinado

### Requirement: Consulta e listagem

O serviço SHALL consultar um canal e listar os seguidos com paginação por
cursor, expondo metadados e momento de atualização.

#### Scenario: Lista paginada

- **WHEN** o cliente lista canais com limite válido
- **THEN** recebe uma página com itens e o próximo cursor

### Requirement: Paginação por cursor

A listagem SHALL ordenar por canal com `limit` padrão 50 e máximo 100 e
`next_cursor` com o último canal da página; `unfollow` MUST NOT tocar o
cache de metadados.

#### Scenario: Segunda página

- **WHEN** o cliente lista com o `next_cursor` da página anterior
- **THEN** recebe os itens seguintes sem repetição

### Requirement: Autorização de newsletters

As operações SHALL exigir dual auth e ownership e MUST responder `403` sem
ownership.

#### Scenario: Listagem sem ownership

- **WHEN** credencial de outro dono lista canais
- **THEN** a resposta é `403`
