## MODIFIED Requirements

### Requirement: Agregação de configurações na leitura

**BREAKING**: as leituras de instância SHALL agregar os blocos `integration` e `settings` na representação pública: `GET /instances/{id}` em `data.instance` e `GET /instances` em `data.instances[]`, com criação e atualização respondendo a mesma representação. `integration.webhook` SHALL refletir a configuração persistida; `integration.chatwoot_config` SHALL ser omitido quando não há configuração persistida. `settings.default_disappearing` SHALL ecoar o último valor aceito por `PUT /instances/{id}/chats/default-disappearing` e SHALL ser omitido quando nunca configurado, sem valor inventado, preservando o valor configurado `"0"`. Os blocos vivos `settings.profile`, `settings.privacy` e `settings.status_privacy` SHALL ser buscados apenas quando `connection.status` é `connected` e SHALL ser omitidos nos demais estados.

#### Scenario: default_disappearing nunca configurado

- **WHEN** a instância nunca passou por `PUT /instances/{id}/chats/default-disappearing`
- **THEN** `settings.default_disappearing` é omitido

#### Scenario: Eco do último valor aceito

- **WHEN** o comando de timer padrão aceita um valor para a instância
- **THEN** as leituras seguintes devolvem esse valor em `settings.default_disappearing`

#### Scenario: Instância conectada

- **WHEN** uma instância com `connection.status` igual a `connected` é lida
- **THEN** `settings.profile`, `settings.privacy` e `settings.status_privacy` vêm preenchidos com os valores atuais da conta

#### Scenario: Criação e atualização com a mesma representação

- **WHEN** a criação ou a atualização de uma instância responde a representação pública
- **THEN** a resposta inclui `integration` e aplica aos opcionais a mesma semântica de omissão das leituras

#### Scenario: Timer desligado explicitamente

- **WHEN** o último comando de timer padrão aceitou duração `0`
- **THEN** `settings.default_disappearing` permanece presente com string `"0"`

#### Scenario: Sem subblocos disponíveis

- **WHEN** não existe timer configurado e nenhum bloco vivo está disponível
- **THEN** `settings` é omitido, preservando a representação obrigatória de conexão e integração

### Requirement: Semântica de blocos na listagem

Na listagem, cada item SHALL ser montado de forma independente: a indisponibilidade de uma fonte MUST omitir apenas o bloco opcional afetado, e uma instância desconectada ou indisponível MUST NOT impedir a resposta nem derrubar os demais itens. A montagem dos blocos na listagem SHALL usar concorrência limitada por requisição, de modo que instâncias lentas não esgotem os recursos do serviço.

#### Scenario: Instância desconectada na listagem

- **WHEN** `GET /instances` inclui instâncias desconectadas
- **THEN** cada item aparece com `integration` preenchido, `settings.default_disappearing` ecoado quando configurado e os blocos vivos omitidos, sem falhar a resposta

#### Scenario: Fonte de um bloco indisponível

- **WHEN** a busca de um bloco falha para uma instância na listagem ou na leitura individual
- **THEN** o bloco opcional afetado é omitido e os demais blocos e itens seguem preenchidos

### Requirement: Coleção completa de instâncias do sistema atual

**BREAKING**: GET /instances SHALL responder `{"data":{"instances":[...]}}` com todas as instâncias autorizadas, ordenadas por criação descendente e identificador descendente em empate. A resposta MUST NOT conter next_cursor nem limitar a quantidade de itens. Sessões user SHALL receber somente as próprias instâncias; sessões admin e a chave global SHALL receber todas; chaves de instância SHALL receber 403. Paginação de servidor SHALL permanecer ausente deste contrato.

#### Scenario: Coleção extensa

- **WHEN** um administrador lista uma coleção com mais de 100 instâncias
- **THEN** recebe todas em uma única resposta, sem next_cursor

#### Scenario: Escopo da conta

- **WHEN** uma sessão user lista instâncias de uma coleção que também contém instâncias de outras contas
- **THEN** recebe todas e somente as próprias instâncias

#### Scenario: Coleção vazia

- **WHEN** a conta não possui instâncias autorizadas
- **THEN** recebe data.instances como array vazio

#### Scenario: Chave de instância

- **WHEN** o cliente tenta listar a coleção com uma instance key
- **THEN** recebe 403
