# wzap-instances Specification

## Purpose

Gerenciar o ciclo de vida das instâncias de WhatsApp (pareamento, estado de conexão, reconexão e remoção) para que o backend opere múltiplas contas isoladas sem lidar com o protocolo.

## Requirements

### Requirement: Criação de instância

**BREAKING**: o serviço SHALL permitir criar uma instância com nome e referência externa opcional, retornando data.instance com identificador, connection.status inicial disconnected, webhook e a instance key em claro uma única vez. O dono é a conta da sessão criadora; na criação pela key global sem sessão, o dono é a conta admin mais antiga, podendo ser sobrescrito pelo parâmetro opcional de dono somente por global/admin. A referência externa, quando informada, MUST ser única. Dono e referência MUST continuar persistidos, mas MUST NOT ser devolvidos na representação pública. Criação acima da cota MUST responder 403 quota_exceeded; somente key global, sessão admin ou sessão user dentro da cota SHALL criar instâncias.

#### Scenario: Criação bem-sucedida

- **WHEN** o cliente autorizado envia nome e referência externa inexistente
- **THEN** recebe 201 com data.instance, conexão disconnected e a key de criação, sem exposição do dono ou referência externa

#### Scenario: Referência externa duplicada

- **WHEN** o cliente autorizado tenta criar uma instância com referência externa já usada
- **THEN** recebe 409 e nenhuma instância é criada

#### Scenario: Criação acima da cota

- **WHEN** um usuário sem saldo de cota tenta criar uma instância
- **THEN** recebe 403 quota_exceeded e nada é criado

#### Scenario: Criação pela key global sem sessão

- **WHEN** a key global cria uma instância sem informar dono
- **THEN** o dono persistido é a conta admin mais antiga, sem aparecer na resposta pública

#### Scenario: Criação com key de instância

- **WHEN** o cliente usa uma instance key para criar instância
- **THEN** recebe 403, pois a key só opera a própria instância

### Requirement: Pareamento por QR

O serviço SHALL iniciar o pareamento de uma instância não conectada, retornando um QR code e sua validade, e MUST transicionar o estado para `pairing`. Após a leitura do QR, o estado MUST virar `connected` com o identificador público da conta registrado.

#### Scenario: Início de pareamento

- **WHEN** o cliente solicita conectar uma instância `disconnected`
- **THEN** o serviço responde com o QR e a expiração, e o estado passa a `pairing`

#### Scenario: QR expirado

- **WHEN** o QR expira sem leitura e o cliente consulta o estado de pareamento
- **THEN** o serviço fornece um novo QR válido

#### Scenario: Pareamento concluído

- **WHEN** o usuário lê o QR no aplicativo
- **THEN** o estado passa a `connected`, o identificador público é registrado e um evento de conexão é publicado

### Requirement: Pareamento por código de telefone

Como alternativa ao QR, o serviço SHALL emitir código de 8 dígitos via
`POST /instances/{id}/pair-phone` quando houver canal de pareamento aberto
(`connect` prévio), sem mudar `connect`/`qr`/`status`/`disconnect` (ver
`wzap-phone-pairing`).

#### Scenario: Código emitido com canal aberto

- **WHEN** o cliente pede código para número válido com canal aberto
- **THEN** a resposta traz `pairing_code` e expiração alinhada ao canal

### Requirement: Instância já conectada

O serviço SHALL ser idempotente ao receber pedido de conexão para instância já logada, sem gerar novo pareamento.

#### Scenario: Conectar instância conectada

- **WHEN** o cliente solicita conectar uma instância `connected`
- **THEN** o serviço responde `connected` sem emitir QR

### Requirement: Reconexão automática

Em queda transitória, o serviço SHALL tentar reconectar com espera crescente e MUST tornar o estado observável durante as tentativas.

#### Scenario: Queda transitória

- **WHEN** a conexão cai por falha de rede temporária
- **THEN** o estado reflete a tentativa de reconexão e volta a `connected` quando restabelecida

### Requirement: Desconexão definitiva

O serviço SHALL encerrar a sessão ao receber pedido de desconexão, remover as credenciais da instância e MUST NOT reconectar automaticamente depois disso.

#### Scenario: Desconectar instância

- **WHEN** o cliente solicita desconectar uma instância conectada
- **THEN** o estado vira `disconnected`, as credenciais são removidas e nenhuma reconexão é tentada

### Requirement: Restrição da conta

Quando a conta for restringida pela plataforma WhatsApp, o serviço SHALL marcar a instância com estado `error`, registrar o motivo e MUST NOT tentar reconexão automática.

#### Scenario: Restrição detectada

- **WHEN** a plataforma informa restrição ou banimento da conta
- **THEN** o estado vira `error` com motivo registrado e não há reconexão automática

### Requirement: Remoção de instância

O serviço SHALL permitir remover uma instância, encerrando a sessão e eliminando dados associados; operações posteriores sobre a instância MUST responder `404`.

#### Scenario: Remoção concluída

- **WHEN** o cliente remove uma instância existente
- **THEN** sessão, mensagens e mídias associadas são eliminadas e consultas seguintes respondem `404`

### Requirement: Restauração no boot

O serviço SHALL restaurar automaticamente as sessões registradas ao iniciar, com concorrência limitada, e MUST refletir o resultado de cada restauração no estado da instância.

#### Scenario: Reinício do serviço

- **WHEN** o serviço reinicia com sessões persistidas
- **THEN** as instâncias voltam a `connected` sem novo pareamento

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

### Requirement: Targeted instance stats

`GET /instances/stats` SHALL accept an optional `instance` UUID/name query. With a target it SHALL return the existing `total` and `by_status` response counting only that authorized instance; without one it SHALL preserve aggregate stats over the authorized collection. Collection authorization MUST remain required, including rejection of instance keys.

#### Scenario: Target by UUID or name
- **WHEN** an authorized collection client selects an accessible instance by either reference
- **THEN** total is one and the correct status bucket is one

#### Scenario: Missing or foreign target
- **WHEN** the target is missing or belongs to another non-admin user's scope
- **THEN** the response is 404 or 403 respectively

#### Scenario: Unfiltered stats
- **WHEN** the query is absent
- **THEN** existing collection totals and status buckets are returned

### Requirement: Persistência exclusiva do vínculo de dispositivo

Reconexão e restauração SHALL carregar as credenciais do dispositivo vinculado exclusivamente à instância. Identidade, conexão e webhook SHALL ter estados persistidos separados sem cópias conflitantes; editar identidade MUST NOT reverter uma transição de conexão concorrente.

#### Scenario: Restaurar sessão vinculada

- **WHEN** o serviço reinicia com credenciais válidas para um dispositivo vinculado
- **THEN** restaura esse dispositivo sem parear outro nem reutilizar o vínculo de outra instância

### Requirement: Nomes válidos únicos de instância

**BREAKING**: All instance names SHALL be globally unique, case-sensitive ASCII strings of 1–64 characters containing letters, digits, hyphens or underscores, with an alphanumeric first and last character. The exact name `stats` and every string interpreted as a UUID SHALL be reserved. Invalid names MUST return `422 invalid_instance_name`; occupied names MUST return `409 instance_name_taken`, without changing the instance or issuing a key. Every persisted instance SHALL satisfy this grammar and uniqueness rule, without exceptions for historical values; renaming MUST preserve its UUID.

#### Scenario: Valid new name
- **WHEN** an authorized client creates `Loja_SP-1` with an available name
- **THEN** the instance is created with that exact name

#### Scenario: Invalid or reserved name
- **WHEN** a client creates or renames to a name outside the grammar, `stats`, or a UUID-shaped string
- **THEN** the response is 422 and no write or key issuance occurs

#### Scenario: Concurrent name claims
- **WHEN** creates or renames concurrently claim the same available exact name
- **THEN** exactly one succeeds and the other receives 409

### Requirement: Resolução de instância por UUID ou nome

Every instance-scoped API path, including the public Chatwoot webhook, SHALL accept either an instance UUID or its exact valid name. UUIDs SHALL take precedence and remain immutable identity for keys, sessions, messages, events, rate limits and idempotency. Missing references MUST return 404. Renaming SHALL immediately change name resolution without changing UUID access.

#### Scenario: Equivalent path references
- **WHEN** a client reads or operates an instance using its UUID or exact name
- **THEN** both address the same instance and preserve the operation's response and authorization

#### Scenario: Rename
- **WHEN** an instance is renamed
- **THEN** the old name no longer resolves, the new name resolves, and its original UUID still resolves

#### Scenario: Missing reference

- **WHEN** a client addresses a missing UUID or name
- **THEN** the response is 404 without ambiguity handling or disclosure of another instance

### Requirement: Coleção completa de instâncias do sistema atual

GET /instances SHALL responder `{"data":{"items":[...]}}` com todas as instâncias autorizadas, ordenadas por criação descendente e identificador descendente em empate. A resposta MUST NOT conter next_cursor nem limitar a quantidade de itens. Sessões user SHALL receber somente as próprias instâncias; sessões admin e a chave global SHALL receber todas; chaves de instância SHALL receber 403. Paginação de servidor SHALL permanecer ausente deste contrato.

#### Scenario: Coleção extensa

- **WHEN** um administrador lista uma coleção com mais de 100 instâncias
- **THEN** recebe todas em uma única resposta, sem next_cursor

#### Scenario: Escopo da conta

- **WHEN** uma sessão user lista instâncias de uma coleção que também contém instâncias de outras contas
- **THEN** recebe todas e somente as próprias instâncias

#### Scenario: Coleção vazia

- **WHEN** a conta não possui instâncias autorizadas
- **THEN** recebe data.items como array vazio

#### Scenario: Chave de instância

- **WHEN** o cliente tenta listar a coleção com uma instance key
- **THEN** recebe 403

### Requirement: Preservação de campos omitidos em PATCH concorrente

Atualizações parciais de uma instância SHALL preservar todo campo omitido, inclusive quando duas atualizações concorrentes alteram identidade ou configuração de webhook. PATCH somente de webhook MUST NOT restaurar nome anterior nem substituir identidade; PATCH parcial de webhook MUST NOT restaurar eventos ou enabled antigos.

#### Scenario: Webhook concorrente com rename
- **WHEN** um PATCH de webhook concorre com rename da mesma instância
- **THEN** ambos os campos solicitados permanecem e o alias antigo continua inválido

#### Scenario: Identidade parcial concorrente
- **WHEN** nome e outro campo de identidade são atualizados por requisições distintas
- **THEN** nenhuma requisição restaura o valor omitido da outra

#### Scenario: Configuração parcial concorrente
- **WHEN** URL, enabled ou events são atualizados por PATCHs distintos
- **THEN** todos os valores explicitamente aceitos são preservados
