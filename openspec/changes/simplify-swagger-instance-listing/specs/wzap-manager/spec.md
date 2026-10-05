## ADDED Requirements

### Requirement: Consulta única da coleção de instâncias

O Manager SHALL obter a coleção completa com uma única chamada GET /instances sem limit ou cursor para a lista de instâncias, métricas do overview e contagem de uso por conta. A lista SHALL manter busca, filtros, ordenação e paginação visual local, sem botão para carregar páginas do servidor.

#### Scenario: Listagem do console

- **WHEN** a conta carrega a lista de instâncias
- **THEN** todas as instâncias autorizadas ficam disponíveis para busca e paginação visual com uma única consulta da coleção

#### Scenario: Uso por conta

- **WHEN** o administrador carrega o uso de instâncias por conta
- **THEN** a contagem usa todos os itens de uma única consulta GET /instances

## MODIFIED Requirements

### Requirement: Overview Home com métricas reais

O overview SHALL exibir stats por status (total, connected, disconnected/pairing, error), gráfico de criação por período e as 5 instâncias mais recentes, derivados de `GET /instances/stats` com fallback para contagem local da listagem quando o endpoint falhar.

#### Scenario: Stats do endpoint

- **WHEN** a conta abre o overview com `GET /instances/stats` saudável
- **THEN** vê total e por-status iguais aos da API, cada card ligando para `/instances`

#### Scenario: Fallback local

- **WHEN** `GET /instances/stats` falha
- **THEN** o overview deriva os mesmos cards da listagem completa obtida em uma chamada e exibe aviso discreto

#### Scenario: Gráfico por período

- **WHEN** a conta troca o período (dia/semana/mês)
- **THEN** o gráfico reagrupa os `created_at` carregados sem nova chamada de série temporal
