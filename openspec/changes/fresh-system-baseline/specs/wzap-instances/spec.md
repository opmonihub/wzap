## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Vínculo exclusivo de dispositivo

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Persistência exclusiva do vínculo de dispositivo".

**Migration:** usar somente a instalação nova e o requisito "Persistência exclusiva do vínculo de dispositivo", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Persistência exclusiva do vínculo de dispositivo" sem depender de cenários históricos deste requisito

### Requirement: URL-safe unique instance names

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Nomes válidos únicos de instância".

**Migration:** usar somente a instalação nova e o requisito "Nomes válidos únicos de instância", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Nomes válidos únicos de instância" sem depender de cenários históricos deste requisito

### Requirement: Instance references by UUID or name

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Resolução de instância por UUID ou nome".

**Migration:** usar somente a instalação nova e o requisito "Resolução de instância por UUID ou nome", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Resolução de instância por UUID ou nome" sem depender de cenários históricos deste requisito

### Requirement: Listagem completa de instâncias

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Coleção completa de instâncias do sistema atual".

**Migration:** usar somente a instalação nova e o requisito "Coleção completa de instâncias do sistema atual", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Coleção completa de instâncias do sistema atual" sem depender de cenários históricos deste requisito
