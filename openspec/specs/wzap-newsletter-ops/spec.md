# wzap-newsletter-ops Specification

## Purpose

Operar canais/newsletters WhatsApp (seguir, silenciar, reações e visualizações) por instância conectada.

## Requirements

### Requirement: Criação de canal

O serviço SHALL criar canal (`title` 1..100, `description` 0..500) e
devolver identificador e metadados, e MUST responder `422` para dados
inválidos e `409` quando desconectada.

#### Scenario: Canal criado

- **WHEN** o cliente cria canal válido em instância conectada
- **THEN** o canal é criado e a resposta traz o identificador

#### Scenario: Título inválido

- **WHEN** o cliente cria canal sem título
- **THEN** a resposta é `422` e nada é criado

### Requirement: Silenciar e marcar visualização

O serviço SHALL silenciar/reativar (`muted: bool`) e marcar mensagens como
vistas (`server_ids[]` não vazio, até 100), e MUST responder `422` para
lista vazia ou acima do limite e `404` para canal desconhecido.

#### Scenario: Canal silenciado

- **WHEN** o cliente silencia canal existente
- **THEN** o mute é aplicado e a resposta confirma

### Requirement: Reação em mensagem de canal

O serviço SHALL reagir a mensagem de canal (`server_id` + `reaction`);
emoji vazio SHALL remover a reação anterior; e MUST responder `422` para
alvo inválido e `404` para mensagem desconhecida.

#### Scenario: Reação removida

- **WHEN** o cliente envia reação vazia para mensagem reagida
- **THEN** a reação é removida e a resposta confirma

### Requirement: Leitura de mensagens do canal

O serviço SHALL listar mensagens e atualizações do canal com paginação por
cursor do upstream (`limit` padrão 50, máximo 100), expondo identificador
de servidor, conteúdo e momento.

#### Scenario: Mensagens paginadas

- **WHEN** o cliente lista mensagens com limite válido
- **THEN** recebe uma página com itens e o próximo cursor

### Requirement: Idempotência e autorização de canal

Escritas SHALL aceitar `Idempotency-Key` (replay, `422` divergente, `409`
concorrência). As operações SHALL exigir dual auth e ownership e MUST
responder `404` em instância inexistente e `403` sem ownership.

#### Scenario: Replay de criação

- **WHEN** o cliente repete a key com o mesmo `title`+`description`
- **THEN** a resposta original é devolvida com indicador de replay
