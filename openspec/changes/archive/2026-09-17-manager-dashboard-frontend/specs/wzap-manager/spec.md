# wzap-manager (delta: manager-dashboard-frontend)

## Purpose

Delta aditivo sobre `openspec/specs/wzap-manager/spec.md`: o console vira o frontend padrão do template dashboard, com overview Home alimentado por métricas reais da API. Nenhum comportamento existente muda; o endpoint novo é aditivo.

## Requirements

### Requirement: Overview Home com métricas reais

O overview SHALL exibir stats por status (total, connected, disconnected/pairing, error), gráfico de criação por período e as 5 instâncias mais recentes, derivados de `GET /instances/stats` com fallback para contagem local da listagem quando o endpoint falhar.

#### Scenario: Stats do endpoint

- **WHEN** a conta abre o overview com `GET /instances/stats` saudável
- **THEN** vê total e por-status iguais aos da API, cada card ligando para `/instances`

#### Scenario: Fallback local

- **WHEN** `GET /instances/stats` falha
- **THEN** o overview deriva os mesmos cards da listagem cursor-acumulada e exibe aviso discreto

#### Scenario: Gráfico por período

- **WHEN** a conta troca o período (dia/semana/mês)
- **THEN** o gráfico reagrupa os `created_at` carregados sem nova chamada de série temporal

### Requirement: Endpoint aditivo de stats

`GET /instances/stats` SHALL responder `{"data":{"total":N,"by_status":{"connected":N,"disconnected":N,"pairing":N,"error":N}}}` respeitando o escopo da sessão (admin vê tudo, user só as próprias); instance keys SHALL receber 403 como nas demais rotas de coleção.

#### Scenario: Stats do operador

- **WHEN** um admin chama `GET /instances/stats`
- **THEN** recebe a contagem de todas as instâncias por status

#### Scenario: Stats do cliente

- **WHEN** um user chama `GET /instances/stats`
- **THEN** recebe a contagem só das instâncias que possui

#### Scenario: Chave de instância

- **WHEN** uma instance key chama `GET /instances/stats`
- **THEN** recebe 403 `forbidden`

### Requirement: Listas e detalhe no padrão do template

As listas SHALL manter dados, filtros, ordenação, seleção e paginação atuais adotando o `ui` de tabela do template customers (bordas arredondadas, header com fundo) e footer com contagem de selecionados + paginação; o detalhe SHALL manter todos os cards atuais com forms centrais `lg:max-w-2xl` e messages como painel lateral no desktop / `USlideover` no mobile.

#### Scenario: Lista preservada

- **WHEN** a conta usa busca, filtro de status, ordenação, seleção ou paginação
- **THEN** o comportamento é o atual, só o visual segue o template

#### Scenario: Detalhe preservado

- **WHEN** a conta abre o detalhe
- **THEN** vê pairing, nome, key, webhook, test-send, messages e danger com os mesmos fluxos, reorganizados no split settings+inbox
