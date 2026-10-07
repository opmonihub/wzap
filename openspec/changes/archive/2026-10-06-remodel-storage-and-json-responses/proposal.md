## Why

O banco mistura identidade, conexão e webhook da instância, e os nomes das colunas não seguem um padrão único. As respostas REST expõem campos internos e exigem uma estrutura por recurso que acompanhe a remodelagem sem alterar o contrato dos eventos.

## What Changes

- Registrar as 14 tabelas aprovadas, com `id` UUID, `created_at`, `updated_at`, nomes em inglês com `snake_case`, booleanos `is_`/`has_` e contadores `_count`.
- Separar `instances`, `instance_connections` e `instance_webhooks`; manter somente `device_jid` como vínculo persistido com o WhatsMeow.
- Estruturar os erros persistidos em código, mensagem e instante da ocorrência.
- Formalizar os vínculos opcionais entre fila, mídia e correlações Chatwoot, respeitando a instância.
- **BREAKING**: organizar respostas REST por recurso; a instância fica em `data.instance` ou `data.items[].instance`, com `connection` e `webhook` aninhados e campos internos ocultos.
- Atualizar Manager, DTOs, documentação e Swagger no mesmo corte de contrato, preservando autenticação, propriedade, quotas e os formatos externos existentes.
- Armazenar arquivos no MinIO, preservar metadados após expiração e migrar arquivos locais mantendo seus UUIDs e integridade.
- Usar exatamente `quay.io/minio/minio:RELEASE.2024-01-13T07-53-03Z-cpuv1`, na rede existente do projeto, com credenciais de ambiente.
- Manter somente eventos pendentes em `event_outbox`; remover após confirmação do JetStream, preservando o `event_id` nas retentativas.
- Converter respostas antigas de idempotência sem reexecutar operações concluídas.

## Capabilities

### New Capabilities

- `wzap-storage-model`: modelo persistido aprovado, unicidade, relações e preservação de identidade durante a migração.
- `wzap-response-contract`: envelopes por recurso, privacidade, erros estruturados e replay compatível de respostas REST.

### Modified Capabilities

- `wzap-instances`: representação pública aninhada e vínculo exclusivo de dispositivo.
- `wzap-media`: armazenamento MinIO, preservação dos metadados e expiração dos objetos.
- `wzap-inbound-events`: outbox somente de pendentes, com confirmação e deduplicação estável.
- `wzap-manager`: consumo dos novos DTOs e contagem de instâncias fornecida pelo backend.
- `wzap-chatwoot-mirror`: mídia pelo armazenamento de objetos e correlações independentes da fila de saída.
- `wzap-chatwoot-inbound`: referência das saídas do Chatwoot aos UUIDs da fila.
- `wzap-chatwoot-config`: nomes curtos aprovados e token protegido.
- `wzap-outbound-messaging`: respostas sob `message`, nomes de envio e erros estruturados.
- `wzap-accounts`: representação de limite e uso de instâncias sem exposição de propriedade nas instâncias.

## Impact

Persistência PostgreSQL, migrações Goose, serviços de instância/mensagem/mídia/eventos/Chatwoot, transporte HTTP, Manager, Swagger, Compose e configuração de produção. A integração de mídia deverá usar o SDK S3 para Go e adaptar os leitores que hoje exigem um arquivo local temporário.

Este registro não é uma execução de migração. `design.md` registra as decisões pendentes e `plan.md` exige fechá-las antes de começar o código. A existência dos artefatos no CLI não significa liberação para aplicar a change.

## Out-of-Scope

- Remodelar tabelas ou credenciais internas do WhatsMeow e do Goose.
- Alterar os eventos NATS/webhooks de versão 1, suas identidades ou seus payloads públicos.
- Criar histórico geral de mensagens recebidas ou histórico de mudanças de estado.
- Copiar a rede `Gacont`, os domínios, as credenciais ou as regras Traefik do exemplo fornecido.
- Trocar silenciosamente a tag MinIO solicitada, implantar produção, remover dados locais ou integrar as alterações de outras changes.

## Pendências conhecidas

A consulta ao registro retornou `no such manifest` para a tag MinIO exata. Tipos/nullabilidade finais, tratamento dos vínculos órfãos/legados, exclusão referencial e matriz de respostas por rota precisam ser concluídos na tarefa 1.1 antes da implementação.
