## ADDED Requirements

### Requirement: Reconhecimento somente do lote importado

Import SHALL reconhecer somente o snapshot efetivamente processado. Chunks recebidos durante import MUST permanecer disponíveis para o próximo lote; falha MUST conservar os dados não processados, incluindo o lote original quando não houve confirmação.

#### Scenario: Chunk durante import
- **WHEN** novo histórico chega depois do snapshot e antes da conclusão bem-sucedida
- **THEN** o novo chunk continua pendente depois da importação

#### Scenario: Import falha
- **WHEN** o processamento do snapshot falha
- **THEN** lote original e novos chunks continuam disponíveis para tentativa posterior

### Requirement: Coleta de histórico condicionada à URI

Sem URI de import, acumuladores, previews e gatilhos de conclusão SHALL permanecer inertes e MUST NOT reter histórico. Com URI configurada SHALL manter o fluxo de acúmulo/import existente.

#### Scenario: URI ausente
- **WHEN** a sessão recebe history-sync sem URI de import configurada
- **THEN** não acumula contatos, conversas ou chunks nem inicia previews/import
