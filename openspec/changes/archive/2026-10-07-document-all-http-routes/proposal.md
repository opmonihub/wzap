## Why

O Swagger gerado dos handlers omite cinco operações Chatwoot e a entrada do Manager. Diversas respostas descrevem apenas o payload interno e não o envelope `data` realmente retornado, prejudicando testes e clientes construídos a partir da documentação.

## What Changes

- Documentar todos os pares método/path explicitamente registrados, incluindo Chatwoot e a entrada pública do Manager.
- Corrigir schemas de sucesso para representar envelopes, arrays, respostas sem corpo, mídia binária e respostas públicas especiais.
- Descrever autenticação por apikey e a alternativa de sessão do Manager, mantendo públicas as superfícies que já são públicas.
- Regenerar os três artefatos Swagger e verificar automaticamente cobertura e contratos de resposta.

## Capabilities

### New Capabilities

Nenhuma.

### Modified Capabilities

- `wzap-operations`: explicitar cobertura de cada método/path e fidelidade dos schemas da documentação interativa.

## Impact

Anotações e testes de `internal/httpapi`, comentários do handler do Manager, arquivos gerados em `docs/` e instruções de geração no README. Nenhuma nova dependência ou alteração de contrato REST/eventos.

## Out-of-Scope

Novos endpoints, alterações no comportamento de autenticação/idempotência, correções independentes de persistência e enumeração de arquivos estáticos ou paths arbitrários da SPA.
