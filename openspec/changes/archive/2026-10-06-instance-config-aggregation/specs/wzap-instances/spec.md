## ADDED Requirements

### Requirement: Agregação de configurações na leitura

As leituras de instância SHALL agregar os blocos `integration` e `settings` na representação pública: `GET /instances/{id}` em `data.instance` e `GET /instances` em `data.items[].instance`, com criação e atualização respondendo a mesma representação. `integration.webhook` SHALL refletir a configuração persistida; `integration.chatwoot_config` SHALL ser `null` quando não há configuração persistida. `settings.default_disappearing` SHALL ecoar o último valor aceito por `PUT /instances/{id}/chats/default-disappearing` e SHALL ser `null` quando nunca configurado, sem valor inventado. Os blocos vivos `settings.profile`, `settings.privacy` e `settings.status_privacy` SHALL ser buscados apenas quando `connection.status` é `connected` e SHALL ser `null` nos demais estados.

#### Scenario: default_disappearing nunca configurado

- **WHEN** a instância nunca passou por `PUT /instances/{id}/chats/default-disappearing`
- **THEN** `settings.default_disappearing` é `null`

#### Scenario: Eco do último valor aceito

- **WHEN** o comando de timer padrão aceita um valor para a instância
- **THEN** as leituras seguintes devolvem esse valor em `settings.default_disappearing`

#### Scenario: Instância conectada

- **WHEN** uma instância com `connection.status` igual a `connected` é lida
- **THEN** `settings.profile`, `settings.privacy` e `settings.status_privacy` vêm preenchidos com os valores atuais da conta

#### Scenario: Criação e atualização com a mesma representação

- **WHEN** a criação ou a atualização de uma instância responde a representação pública
- **THEN** a resposta inclui `integration` e `settings` com a mesma semântica de `null` das leituras

### Requirement: Semântica de blocos na listagem

Na listagem, cada item SHALL ser montado de forma independente: a indisponibilidade de uma fonte MUST produzir `null` apenas no bloco afetado, e uma instância desconectada ou indisponível MUST NOT impedir a resposta nem derrubar os demais itens. A montagem dos blocos na listagem SHALL usar concorrência limitada por requisição, de modo que instâncias lentas não esgotem os recursos do serviço.

#### Scenario: Instância desconectada na listagem

- **WHEN** `GET /instances` inclui instâncias desconectadas
- **THEN** cada item aparece com `integration` preenchido, `settings.default_disappearing` ecoado e os blocos vivos `null`, sem falhar a resposta

#### Scenario: Fonte de um bloco indisponível

- **WHEN** a busca de um bloco falha para uma instância na listagem ou na leitura individual
- **THEN** o bloco afetado é `null` e os demais blocos e itens seguem preenchidos
