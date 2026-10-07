# wzap-response-contract Specification

## Purpose

Padronizar a representação pública dos recursos REST com agrupamento por entidade, privacidade e replay compatível, preservando os envelopes HTTP e o contrato distinto de eventos.

## Requirements

### Requirement: Recursos em envelopes próprios

**BREAKING**: respostas JSON de recursos SHALL manter o envelope data e organizar objetos pela entidade: data.instance para instância individual e data.items[].instance para suas coleções. Outros recursos SHALL usar suas próprias chaves, como message, user e group. Arrays vazios MUST ser representados por listas vazias. O envelope de erro e X-Request-Id SHALL ser preservados.

#### Scenario: Listagem de instâncias

- **WHEN** um cliente autorizado lista instâncias
- **THEN** cada item contém instance com connection, integration e settings aninhados, seguindo o exemplo do design

#### Scenario: Coleção vazia

- **WHEN** a consulta não encontra recursos no escopo autorizado
- **THEN** a coleção contém items como lista vazia

### Requirement: Privacidade da representação pública

Respostas públicas de instância MUST omitir whatsapp_jid, device_jid, external_ref, owner_user_id e hashes, preservando referência e propriedade no banco e nos fluxos autorizados de escrita. Os blocos integration e settings MUST omitir referência externa, proprietário, JIDs internos e hashes. Senhas e tokens MUST NOT ser expostos; o token do Chatwoot continua write-only e MUST NOT aparecer em nenhuma leitura, inclusive na agregação. A chave de instância SHALL ser retornada apenas nas respostas específicas de criação e rotação.

#### Scenario: Leitura de instância

- **WHEN** o operador consulta uma instância
- **THEN** obtém seus dados públicos aninhados sem os campos internos ou a chave de acesso

#### Scenario: Leitura dos blocos agregados

- **WHEN** o operador consulta uma instância com integração Chatwoot configurada
- **THEN** obtém integration.chatwoot_config sem o token e sem referência externa, proprietário, JIDs internos ou hashes

### Requirement: Erros públicos estruturados

Erros de conexão e envio SHALL ser objetos com code, message e occurred_at ou null quando ausentes. Um erro legado com data desconhecida SHALL conservar sua mensagem com code legacy_error e occurred_at null.

#### Scenario: Erro legado

- **WHEN** uma instância possui somente um texto legado de falha
- **THEN** sua resposta usa legacy_error, preserva o texto e informa occurred_at null

### Requirement: Replay sem repetir efeitos

Uma resposta idempotente legada SHALL ser adaptada ao contrato atual conservando identificador, status HTTP e resultado da operação. A conversão MUST NOT executar a operação novamente, eliminar o cache indiscriminadamente ou devolver campos públicos removidos.

#### Scenario: Mensagem aceita antes do corte

- **WHEN** uma requisição autorizada repete a mesma chave e conteúdo de uma mensagem aceita antes da remodelagem
- **THEN** recebe o resultado convertido com o mesmo identificador sem um segundo envio

### Requirement: Contratos externos preservados

Downloads, respostas sem corpo, HTML e confirmações externas SHALL conservar seus formatos próprios. A remodelagem REST MUST preservar event_version 1, IDs, payloads e strings de erro/motivo dos eventos NATS e webhooks.

#### Scenario: Evento após a remodelagem

- **WHEN** um evento é produzido após o corte do JSON REST
- **THEN** consumidores da versão 1 recebem o mesmo contrato de evento

### Requirement: Blocos de integração e configurações da instância

**BREAKING**: a representação pública da instância SHALL expor os blocos `integration` (com `webhook` e `chatwoot_config`) e `settings` (com `default_disappearing`, `profile`, `privacy` e `status_privacy`); o objeto `webhook` MUST sair da raiz de `instance` e existir apenas em `integration.webhook`. `integration.webhook` SHALL refletir a configuração persistida. `integration.chatwoot_config` SHALL repetir a semântica de leitura da configuração Chatwoot por instância, sem `instance_id`, e MUST NOT conter o token. Os blocos `settings.profile`, `settings.privacy` e `settings.status_privacy` devem ser objetos ou `null`, conforme disponibilidade.

#### Scenario: Webhook aninhado em integration

- **WHEN** um cliente autorizado lê uma instância em `GET /instances/{id}` ou `GET /instances`
- **THEN** a configuração de webhook aparece somente em `integration.webhook` e não existe chave `webhook` na raiz de `instance`

#### Scenario: Bloco indisponível

- **WHEN** a fonte de um bloco não está disponível para a instância lida
- **THEN** apenas esse bloco é `null` e a resposta continua sendo entregue com os demais blocos preenchidos

#### Scenario: Configuração Chatwoot ausente

- **WHEN** a instância nunca teve configuração Chatwoot persistida
- **THEN** `integration.chatwoot_config` é `null`