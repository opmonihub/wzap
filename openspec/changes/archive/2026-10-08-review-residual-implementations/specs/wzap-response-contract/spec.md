## ADDED Requirements

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
