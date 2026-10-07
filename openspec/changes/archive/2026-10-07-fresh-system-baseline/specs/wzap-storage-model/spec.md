## ADDED Requirements

### Requirement: Instalação inicial sobre banco vazio

**BREAKING:** a instalação SHALL criar diretamente o modelo atual sobre banco vazio, sem transformação, adoção, reconciliação ou importação de dados anteriores. A instalação repetida sobre a mesma base atual MUST preservar os recursos já criados. Identidades, conexão, webhook e configurações de chat SHALL ter persistência separada; schemas administrados por bibliotecas MUST permanecer sob seus próprios contratos.

#### Scenario: Instalação vazia

- **WHEN** o operador inicializa uma instalação sem dados
- **THEN** o modelo atual fica pronto para contas, instâncias, mensagens, mídia e integrações sem etapas de upgrade histórico

#### Scenario: Inicialização repetida

- **WHEN** o operador reinicia uma instalação criada pela base atual
- **THEN** as migrações já aplicadas não recriam recursos nem alteram seus identificadores

#### Scenario: Bibliotecas na instalação nova

- **WHEN** a instalação cria a persistência de sessões
- **THEN** as bibliotecas mantêm seus contratos de armazenamento sem transformação histórica pelo serviço

### Requirement: Integridade de identidade e ownership

Toda instância MUST ter exatamente um proprietário existente e um nome válido globalmente único. Claims concorrentes do mesmo nome MUST permitir apenas um sucesso. A remoção de proprietário com instâncias MUST ser recusada, preservando os recursos. Vínculos de mídia e correlação de mensagens MUST respeitar a mesma instância.

#### Scenario: Criação sem proprietário

- **WHEN** uma criação não consegue determinar um proprietário existente
- **THEN** a instância não é persistida e nenhuma key é emitida

#### Scenario: Mesmo nome concorrente

- **WHEN** duas criações ou renomeações disputam o mesmo nome exato
- **THEN** apenas uma é persistida e a outra responde 409 instance_name_taken

#### Scenario: Proprietário com recursos

- **WHEN** é solicitada a remoção de uma conta dona de instâncias
- **THEN** a operação responde 409 sem remover a conta ou as instâncias

## REMOVED Requirements

### Requirement: Identidades preservadas durante a remodelagem

**Reason:** a instalação nova substitui o upgrade histórico.

**Migration:** começar com banco vazio; dados e identificadores anteriores não são convertidos nem importados.

#### Scenario: Base nova

- **WHEN** o serviço é instalado em banco vazio
- **THEN** cria diretamente recursos do modelo vigente sem preservar identidades de outra instalação

### Requirement: Migração verificável sem fabricação de dados

**Reason:** relatórios de remodelagem e gates de reconciliação existem apenas para dados anteriores.

**Migration:** usar instalação vazia, sem relatório de remodelagem ou resolução de conflitos de corte.

#### Scenario: Sem corte histórico

- **WHEN** uma instalação nova é inicializada
- **THEN** nenhuma etapa de relatório ou reconciliação histórica é exigida
