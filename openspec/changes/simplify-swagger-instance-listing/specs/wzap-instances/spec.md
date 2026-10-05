## ADDED Requirements

### Requirement: Listagem completa de instâncias

GET /instances SHALL responder `{"data":{"items":[...]}}` com todas as instâncias autorizadas, ordenadas por criação descendente e identificador descendente em empate. A resposta MUST NOT conter next_cursor nem limitar a quantidade de itens. Sessões user SHALL receber somente as próprias instâncias; sessões admin e a chave global SHALL receber todas; chaves de instância SHALL receber 403. Parâmetros antigos limit e cursor SHALL ser ignorados.

#### Scenario: Coleção maior que o antigo limite

- **WHEN** um administrador lista uma coleção com mais de 100 instâncias
- **THEN** recebe todas em uma única resposta, sem next_cursor

#### Scenario: Escopo da conta

- **WHEN** uma sessão user lista instâncias de uma coleção que também contém instâncias de outras contas
- **THEN** recebe todas e somente as próprias instâncias

#### Scenario: Parâmetros antigos

- **WHEN** o cliente chama GET /instances com limit ou cursor antigos, incluindo valores inválidos
- **THEN** recebe a mesma listagem completa autorizada sem erro de paginação

#### Scenario: Coleção vazia

- **WHEN** a conta não possui instâncias autorizadas
- **THEN** recebe data.items como array vazio

#### Scenario: Chave de instância

- **WHEN** o cliente tenta listar a coleção com uma instance key
- **THEN** recebe 403
