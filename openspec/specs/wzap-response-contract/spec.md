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

### Requirement: Erros estruturados do contrato vigente

**BREAKING:** erros de conexão e envio SHALL ser objetos com code, message e occurred_at, ou null quando não houver erro. Falhas produzidas pelo sistema atual MUST registrar código específico, mensagem e instante de ocorrência, sem inferência de código a partir de texto histórico. Dados internos e segredos MUST NOT aparecer no erro público.

#### Scenario: Falha atual

- **WHEN** uma operação de conexão ou envio registra uma falha
- **THEN** a resposta apresenta o código, mensagem e instante registrados para aquela falha

#### Scenario: Sem falha registrada

- **WHEN** uma instância ou mensagem não tem erro registrado
- **THEN** last_error é null

### Requirement: Replay do resultado atual armazenado

**BREAKING:** a resposta idempotente SHALL reproduzir o status HTTP e o corpo armazenados pelo contrato atual, sem conversão de formato, leitura substitutiva do recurso ou repetição da operação. A autorização atual MUST ser verificada antes do replay; identificadores, headers de correlação e indicador de replay SHALL preservar sua semântica. Corrupção de um resultado armazenado MUST produzir erro interno sem reexecutar a operação.

#### Scenario: Resultado armazenado

- **WHEN** um cliente atualmente autorizado repete a chave e conteúdo de uma operação concluída
- **THEN** recebe o mesmo status HTTP e corpo armazenado com indicador de replay, sem segundo efeito

#### Scenario: Estado posterior do recurso

- **WHEN** o recurso muda depois de a resposta original ser armazenada e ocorre replay autorizado
- **THEN** a resposta mantém o resultado original sem reconstruí-lo a partir do estado novo

#### Scenario: Autorização atual insuficiente

- **WHEN** um cliente sem escopo atual tenta obter uma resposta armazenada
- **THEN** recebe 403 sem exposição do resultado

### Requirement: Identidade efetiva e integridade da requisição idempotente

Escritas POST, PUT e PATCH explicitamente idempotentes SHALL aplicar replay, 422 para conteúdo divergente e 409 em progresso. O alvo concreto além da instância SHALL participar da identidade da requisição; UUID/nome da mesma instância SHALL compartilhar replay. Corpos JSON SHALL conter exatamente um valor dentro do limite; campos multipart efetivamente consumidos SHALL corresponder ao conteúdo identificado para replay, sem interferência de query.

#### Scenario: PUT ou PATCH repetido
- **WHEN** a escrita é repetida com mesma chave, alvo e conteúdo
- **THEN** a resposta é reproduzida sem repetir a operação

#### Scenario: Recurso distinto na mesma instância
- **WHEN** a chave é reutilizada com mesmo corpo para grupo ou canal diferente
- **THEN** a resposta é 422 e o segundo recurso não é alterado

#### Scenario: JSON com conteúdo extra
- **WHEN** o primeiro valor JSON é seguido de segundo valor ou lixo
- **THEN** a resposta é 400 e nenhum efeito é executado

#### Scenario: JSON acima do limite com padding
- **WHEN** um JSON curto é seguido de espaços que ultrapassam o limite de corpo
- **THEN** a resposta é 413 e nenhum efeito é executado

#### Scenario: Campo multipart e query divergentes
- **WHEN** um campo definido no multipart também aparece com outro valor na query
- **THEN** o valor do formulário é o utilizado pela operação e pelo replay

#### Scenario: Campos multipart repetidos
- **WHEN** mudar a ordem dos campos repetidos modifica o valor efetivamente consumido
- **THEN** o conteúdo é identificado como divergente ou a entrada ambígua é recusada sem executar efeito

### Requirement: Falha segura do replay corrompido

Replay SHALL reproduzir literalmente status e bytes íntegros do envelope armazenado. Status inválido, JSON inválido ou envelope corrompido MUST responder 500 sem reexecutar efeito, liberar chave ou converter o formato.

#### Scenario: Envelope truncado
- **WHEN** o registro concluído contém JSON truncado ou corpo vazio
- **THEN** a resposta é 500, a chave permanece e a operação não é repetida

#### Scenario: Envelope válido
- **WHEN** o registro concluído contém status e envelope íntegros
- **THEN** status e corpo permanecem byte a byte iguais
