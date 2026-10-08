## MODIFIED Requirements

### Requirement: Gestão de instâncias

O console SHALL apresentar as instâncias em tabela padronizada com ordenação, busca e paginação client-side, mantendo criar, editar, desconectar e remover conforme o escopo da conta. A remoção SHALL exigir confirmação digitada com o nome. **BREAKING**: o console SHALL consumir instâncias diretas em `data.instances[]` e `data.instance` nos endpoints individuais, com connection, integration e settings quando disponível; o webhook SHALL ser lido de integration.webhook e os blocos ausentes SHALL ser tratados como indisponíveis, sem falhar a listagem nem o detalhe. A busca SHALL usar os campos públicos disponíveis. Referência externa, proprietário e JIDs internos MUST NOT aparecer na representação pública nem ser apagados por campos vazios enviados automaticamente pelo formulário.

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

#### Scenario: Bloco indisponível no console

- **WHEN** a conta abre uma instância desconectada ou cujo bloco foi omitido
- **THEN** os painéis de integração e configurações mostram os blocos preenchidos e tratam os blocos ausentes como indisponíveis, sem erro

### Requirement: Contrato vigente nas mensagens e tipos

**BREAKING**: o Manager SHALL representar exclusivamente os DTOs atuais, com coleções nomeadas, elementos diretos e propriedades opcionais ausentes, bem como os erros produzidos pelo contrato vigente, sem textos de preservação histórica ou interpretação de envelopes antigos. Falhas atuais MUST continuar visíveis com a mensagem retornada pela API, respeitando os escopos e a privacidade.

#### Scenario: Falha atual exibida

- **WHEN** uma operação atual retorna um erro estruturado
- **THEN** o Manager informa a falha e a mensagem do servidor sem inventar tradução de erro histórico

#### Scenario: Formulário de nome

- **WHEN** o operador abre criação ou edição de instância
- **THEN** as orientações descrevem somente nomes válidos e conflitos do contrato atual

#### Scenario: Coleções nomeadas no console

- **WHEN** o console carrega instâncias, usuários, grupos, mensagens, canais ou publicações próprias de status
- **THEN** consome respectivamente `instances`, `users`, `groups`, `messages`, `channels` e `statuses` sem depender de `items` ou wrappers individuais

#### Scenario: Opcionais ausentes

- **WHEN** uma instância ou mensagem possui propriedades opcionais omitidas, incluindo `settings`, datas, foto ou erro
- **THEN** o console apresenta indisponibilidade ou ausência corretamente sem falhar o carregamento

#### Scenario: Fim da paginação

- **WHEN** uma resposta paginada omite `next_cursor`
- **THEN** o console reconhece o fim das páginas sem solicitar uma continuação inexistente

#### Scenario: Valores zero exibidos

- **WHEN** a API retorna flags desativadas, contadores zero ou timer `"0"`
- **THEN** o console preserva esses valores sem interpretá-los como ausência
