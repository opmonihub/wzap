## ADDED Requirements

### Requirement: Contrato vigente nas mensagens e tipos

O Manager SHALL representar exclusivamente os DTOs atuais e os erros produzidos pelo contrato vigente, sem textos de preservação histórica ou interpretação de envelopes antigos. Falhas atuais MUST continuar visíveis com a mensagem retornada pela API, respeitando os escopos e a privacidade.

#### Scenario: Falha atual exibida

- **WHEN** uma operação atual retorna um erro estruturado
- **THEN** o Manager informa a falha e a mensagem do servidor sem inventar tradução de erro histórico

#### Scenario: Formulário de nome

- **WHEN** o operador abre criação ou edição de instância
- **THEN** as orientações descrevem somente nomes válidos e conflitos do contrato atual

### Requirement: Validação atual de nomes nos formulários

Create, edit and overview forms SHALL explain and validate the same name grammar and reserved names as the API. Every submitted name SHALL satisfy the current grammar; forms MUST NOT offer historical-name exceptions, trimming or silent renaming. Name-conflict errors SHALL be distinguished from external-reference conflicts.

#### Scenario: Invalid new name
- **WHEN** a user enters spaces, accents, invalid punctuation, an overlong value or a reserved name
- **THEN** the form explains the rule and prevents submission

#### Scenario: Name already occupied
- **WHEN** the API returns instance_name_taken
- **THEN** the form reports the instance-name conflict instead of an external-reference conflict

## REMOVED Requirements

### Requirement: Instance name form validation

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Validação atual de nomes nos formulários".

**Migration:** usar somente a instalação nova e o requisito "Validação atual de nomes nos formulários", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Validação atual de nomes nos formulários" sem depender de cenários históricos deste requisito
