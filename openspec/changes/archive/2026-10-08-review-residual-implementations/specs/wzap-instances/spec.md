## ADDED Requirements

### Requirement: Preservação de campos omitidos em PATCH concorrente

Atualizações parciais de uma instância SHALL preservar todo campo omitido, inclusive quando duas atualizações concorrentes alteram identidade ou configuração de webhook. PATCH somente de webhook MUST NOT restaurar nome anterior nem substituir identidade; PATCH parcial de webhook MUST NOT restaurar eventos ou enabled antigos.

#### Scenario: Webhook concorrente com rename
- **WHEN** um PATCH de webhook concorre com rename da mesma instância
- **THEN** ambos os campos solicitados permanecem e o alias antigo continua inválido

#### Scenario: Identidade parcial concorrente
- **WHEN** nome e outro campo de identidade são atualizados por requisições distintas
- **THEN** nenhuma requisição restaura o valor omitido da outra

#### Scenario: Configuração parcial concorrente
- **WHEN** URL, enabled ou events são atualizados por PATCHs distintos
- **THEN** todos os valores explicitamente aceitos são preservados
