## MODIFIED Requirements

### Requirement: Gestão de instâncias

O console SHALL apresentar as instâncias em tabela padronizada com ordenação, busca e paginação client-side, mantendo criar, editar, desconectar e remover conforme o escopo da conta. A remoção SHALL exigir confirmação digitada com o nome. **BREAKING**: o console SHALL consumir a representação instance aninhada, com connection e webhook; a busca SHALL usar os campos públicos disponíveis. Referência externa, proprietário e JIDs internos MUST NOT aparecer na representação pública nem ser apagados por campos vazios enviados automaticamente pelo formulário.

#### Scenario: Localizar instância na tabela

- **WHEN** a conta digita parte do nome na busca
- **THEN** a tabela filtra as instâncias carregadas sem nova chamada à API

#### Scenario: Ordenar instâncias

- **WHEN** a conta ordena nome ou estado
- **THEN** as linhas são reordenadas usando name e connection.status

#### Scenario: Paginar instâncias

- **WHEN** a conta navega entre páginas
- **THEN** vê a fatia dos registros carregados mantendo o filtro

#### Scenario: Criar instância

- **WHEN** a conta cria dentro da cota
- **THEN** a instância aparece disconnected com sua key exibida uma vez para cópia

#### Scenario: Remoção com confirmação

- **WHEN** a conta remove uma instância e digita o nome corretamente
- **THEN** ela é removida; com nome divergente, nada acontece

#### Scenario: Cota excedida

- **WHEN** a conta tenta criar acima da cota
- **THEN** o console exibe o erro de cota sem criar nada

## ADDED Requirements

### Requirement: Uso de instâncias fornecido pelo backend

O Manager SHALL usar a contagem de instâncias fornecida pelo backend para representar consumo de quota. Ocultar o dono nos DTOs de instância MUST NOT fazer o consumo aparecer como zero.

#### Scenario: Usuário com instâncias desconectadas

- **WHEN** o Manager apresenta o limite e uso de uma conta com instâncias desconectadas
- **THEN** mostra a contagem retornada pelo backend e mantém a regra de quota existente
