## MODIFIED Requirements

### Requirement: Seed do primeiro admin

Quando nenhuma conta existir, o serviço SHALL criar a conta `admin` inicial a
partir da configuração de ambiente no boot; sem essa configuração, nenhuma
conta SHALL ser criada automaticamente. O seed MUST criar somente a conta inicial, sem adotar, alterar ou reparar ownership de instâncias.

#### Scenario: Boot com seed configurado

- **WHEN** o serviço inicia sem contas e com email/senha de admin configurados
- **THEN** a conta `admin` existe e pode autenticar-se

#### Scenario: Boot sem seed

- **WHEN** o serviço inicia sem contas e sem a configuração de seed
- **THEN** nenhuma conta é criada e o login segue respondendo `401`

#### Scenario: Seed já realizado

- **WHEN** o serviço inicia com contas já existentes
- **THEN** não cria outro admin nem altera os donos de instâncias

