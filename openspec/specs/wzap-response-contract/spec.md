# wzap-response-contract Specification

## Purpose

Padronizar a representação pública dos recursos REST com agrupamento por entidade, privacidade e replay compatível, preservando os envelopes HTTP e o contrato distinto de eventos.

## Requirements

### Requirement: Recursos em envelopes próprios

**BREAKING**: respostas JSON de recursos SHALL manter o envelope `data`; leituras e escritas individuais SHALL manter suas chaves de entidade, como `data.instance`, `data.message`, `data.user`, `data.group` e `data.channel`. Coleções SHALL usar nomes plurais e elementos diretos, sem `items` nem wrappers individuais, com estes nomes: `GET /instances` → `instances`; `GET /users` → `users`; `GET /instances/{id}/groups` → `groups`; `GET /instances/{instance}/messages` → `messages`; `GET /instances/{id}/newsletters` → `channels`; `GET /instances/{id}/newsletters/{channel}/messages` e `GET /instances/{id}/newsletters/{channel}/updates` → `messages`; `GET /instances/{id}/status/updates` → `statuses`; `POST /instances/{id}/contacts/check` → `contacts`; `GET /instances/{id}/blocklist` → `blocked_jids` (strings de JID). Todas as coleções obrigatórias e arrays obrigatórios aninhados MUST estar presentes como arrays, inclusive `[]` quando vazios, nunca `null`. `next_cursor` SHALL aparecer junto da coleção dentro de `data` somente quando houver próxima página. O envelope de erro e `X-Request-Id` SHALL ser preservados. A ordem das propriedades de objetos JSON MUST NOT ser requisito de contrato; a ordem dos registros SHALL conservar o comportamento vigente.

#### Scenario: Listagem de instâncias

- **WHEN** um cliente autorizado lista instâncias
- **THEN** cada elemento de `data.instances` contém diretamente os campos públicos da instância, incluindo `connection`, `integration` e `settings` quando disponível, sem wrapper `instance`

#### Scenario: Coleção vazia

- **WHEN** a consulta não encontra recursos no escopo autorizado
- **THEN** a chave específica da coleção permanece presente com `[]`, sem `items`, omissão ou `null`

#### Scenario: Arrays obrigatórios aninhados vazios

- **WHEN** um recurso não possui participantes, eventos de webhook, JIDs de privacidade ou JIDs ignorados
- **THEN** seus arrays obrigatórios permanecem presentes com `[]`

#### Scenario: Página intermediária

- **WHEN** a consulta paginada possui próxima página
- **THEN** `data.next_cursor` contém o cursor válido junto da coleção

#### Scenario: Última página

- **WHEN** a consulta paginada não possui próxima página
- **THEN** a coleção permanece presente e `next_cursor` é omitido

#### Scenario: Nomes correspondentes aos recursos

- **WHEN** o cliente consulta canais, status próprios ou atualizações de canal
- **THEN** recebe respectivamente `data.channels` com metadados, `data.statuses` com publicações próprias e `data.messages` com mensagens de canal, conservando as rotas atuais

#### Scenario: Reordenação de propriedades

- **WHEN** dois objetos de resposta possuem os mesmos campos e valores em ordens textuais diferentes
- **THEN** representam o mesmo contrato JSON

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

**BREAKING**: a representação pública da instância SHALL conter `connection` e `integration` obrigatórios, com `integration.webhook` obrigatório e `integration.chatwoot_config` opcional, e SHALL incluir `settings` somente quando ao menos um de seus subblocos `default_disappearing`, `profile`, `privacy` e `status_privacy` estiver disponível. O webhook MUST existir apenas em `integration.webhook`, refletindo sua configuração persistida, inclusive quando desativado. O objeto Chatwoot persistido SHALL conservar sua configuração mesmo desativada, sem `instance_id` na cópia aninhada e sem token; ausência de configuração ou indisponibilidade de sua fonte SHALL omitir apenas `chatwoot_config`. A leitura standalone de Chatwoot SHALL conservar a resposta padrão desativada quando não existe configuração persistida. Subblocos opcionais de `settings` sem configuração ou disponibilidade SHALL ser omitidos, sem `null` ou valores inventados.

#### Scenario: Webhook aninhado em integration

- **WHEN** um cliente autorizado lê uma instância em `GET /instances/{id}` ou `GET /instances`
- **THEN** a configuração de webhook aparece somente em `integration.webhook` e não existe chave `webhook` na raiz da instância

#### Scenario: Bloco indisponível

- **WHEN** a fonte de um bloco opcional não está disponível para a instância lida
- **THEN** apenas esse bloco é omitido e a resposta continua sendo entregue com os demais blocos preenchidos

#### Scenario: Configuração Chatwoot ausente

- **WHEN** a instância nunca teve configuração Chatwoot persistida
- **THEN** `integration.chatwoot_config` é omitido

#### Scenario: Webhook desativado

- **WHEN** a configuração de webhook está desativada
- **THEN** `integration.webhook` permanece presente com `enabled` igual a `false`, seus eventos e a URL somente quando disponível

#### Scenario: Configuração Chatwoot desativada persistida

- **WHEN** existe configuração Chatwoot persistida com integração desativada
- **THEN** `integration.chatwoot_config` permanece presente, preservando flags `false`, `import_days` igual a `0`, `ignored_jids` como array e defaults efetivos

#### Scenario: Nenhuma configuração disponível

- **WHEN** nenhum dos quatro subblocos de configurações está disponível
- **THEN** `settings` é omitido e `connection` e `integration.webhook` permanecem presentes

### Requirement: Erros estruturados do contrato vigente

**BREAKING:** erros de conexão e envio SHALL ser objetos com `code` e `message`, incluindo `occurred_at` somente quando conhecido; `last_error` SHALL ser omitido quando não houver erro. Falhas produzidas pelo sistema atual MUST registrar código específico, mensagem e instante de ocorrência, sem inferência de código a partir de texto histórico. Dados internos e segredos MUST NOT aparecer no erro público.

#### Scenario: Falha atual

- **WHEN** uma operação de conexão ou envio registra uma falha
- **THEN** a resposta apresenta o código, mensagem e instante registrados para aquela falha

#### Scenario: Sem falha registrada

- **WHEN** uma instância ou mensagem não tem erro registrado
- **THEN** `last_error` é omitido

#### Scenario: Instante da falha desconhecido

- **WHEN** há erro estruturado sem instante conhecido
- **THEN** código e mensagem permanecem presentes e `occurred_at` é omitido

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

### Requirement: Presença e valores da representação compartilhada

**BREAKING**: recursos das coleções e suas representações compartilhadas SHALL omitir campos opcionais ausentes, preservando todos os campos obrigatórios e valores zero semanticamente válidos, incluindo `false`, `0`, `"0"` e `[]`. A mesma representação pública SHALL ser usada na listagem e no detalhe para o mesmo estado e contexto. Respostas resumidas de aceite e campos exclusivos de criação SHALL conservar suas diferenças contextuais. Datas opcionais desconhecidas SHALL ser omitidas; datas obrigatórias de instâncias e mensagens SHALL permanecer presentes. Datas de atualização de grupos e canais SHALL ser omitidas quando não existir atualização de metadados conhecida. URLs e identificadores opcionais sem valor, inclusive URL de webhook, foto de perfil, URL e account_id do Chatwoot, SHALL ser omitidos; conteúdo vazio semanticamente válido, como status_text, MUST ser preservado. Metadados opcionais organization e logo do Chatwoot SHALL ser omitidos quando ausentes, conservando os defaults efetivos dos demais campos.

#### Scenario: Valores zero válidos

- **WHEN** um recurso possui flags obrigatórias desativadas, contadores zero, duração configurada como `"0"` ou arrays obrigatórios vazios
- **THEN** os campos permanecem presentes com seus valores e tipos originais

#### Scenario: Ausência de campos opcionais

- **WHEN** uma instância ou mensagem não possui identificador opcional, URL opcional, erro ou marco temporal opcional
- **THEN** apenas os campos opcionais ausentes são omitidos, sem `null`

#### Scenario: Equivalência entre lista e detalhe

- **WHEN** um recurso de mesmo estado e contexto aparece na listagem e na consulta individual
- **THEN** os objetos do recurso possuem os mesmos campos públicos, valores e regras de presença

#### Scenario: Atualização de metadados desconhecida

- **WHEN** um grupo ou canal não possui data conhecida de atualização dos metadados
- **THEN** `updated_at` é omitido sem timestamp inventado

#### Scenario: Recado vazio

- **WHEN** o perfil disponível possui recado explicitamente vazio
- **THEN** `status_text` permanece presente com string vazia e a ausência de foto omite apenas `photo_url`

#### Scenario: Coleção opcional ausente

- **WHEN** uma representação compartilhada possuir coleção opcional sem valor
- **THEN** esse campo é omitido, mantendo distinta a representação das coleções obrigatórias vazias

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
