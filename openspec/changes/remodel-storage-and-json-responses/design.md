## Context

Ver `proposal.md` para a motivação e `brainstorm.md` para o registro das aprovações. Este documento consolida o desenho aprovado; a tarefa 1.1 completa o plano de execução antes de alterar código ou aplicar migrações.

O estado observado inclui 12 tabelas próprias, 17 do WhatsMeow e a tabela de versões Goose. A migração `00007_instance_device_jid.sql` está aplicada no banco local, mas é uma alteração ainda não rastreada no Git: sua integração deve ser coordenada com a change de origem. A mídia atual usa arquivos em `WZAP_DATA_DIR`; o cleaner elimina arquivo e registro. A fila contém apenas saídas. O mirror cria correlações para entradas e edições, e o webhook do Chatwoot hoje descarta os UUIDs retornados pelo enfileiramento.

## Goals / Non-Goals

**Goals:** registrar o modelo aprovado e a resposta pública; preservar UUIDs, dados recuperáveis, ownership, quotas, identidade de eventos e replay de requisições; separar responsabilidades sem duplicar estado.

**Non-Goals:** ver `proposal.md`; esta etapa documental não aplica DDL, não implanta infraestrutura e não finaliza decisões de execução ainda abertas.

## Decisions

### 1. Convenções globais

- `id UUID PRIMARY KEY`, `created_at` e `updated_at` em todas as tabelas próprias. Atualização do registro atualiza `updated_at`; criação inicializa ambas as datas.
- Inglês e `snake_case`; referências por entidade/papel; booleanos `is_`/`has_`; contadores `_count`.
- Datas de instantes usam `timestamptz`; transporte utiliza UTC. Preservar datas existentes e não inferir ocorrência de erro por `updated_at`.
- Tipos de base herdados: JIDs/chaves/telefones e hashes são texto; flags booleanas; payloads/envelopes/respostas JSONB; listas de eventos/JIDs `text[]`; contagens inteiras; bytes e IDs de mensagens/conversas/inboxes Chatwoot `bigint`. `chatwoot_configs.account_id` é uma referência externa textual no schema atual, não uma FK UUID local.
- A lista de nomes está aprovada. Tipos finais, nullabilidade, defaults, enum SQL e ações referenciais devem ter uma matriz explícita na tarefa 1.1; estes tipos de base não autorizam conversões destrutivas.
- Preservar UUIDs atuais. Adicionar UUID às tabelas hoje com PK natural/composto e manter a combinação original única; converter o inteiro de dead letters sem alterar `event_id`.

Alternativa descartada: aplicar as regras às tabelas mantidas pela biblioteca e pelo migrador. Esses schemas continuam sob seus próprios contratos.

### 2. As 14 tabelas aprovadas

Todas incluem **`id`, `created_at`, `updated_at`**. A ordem abaixo não determina a ordem das migrações.

| Tabela final | Demais colunas |
|---|---|
| `instances` | `name`, `external_ref`, `owner_user_id`, `api_key_hash` |
| `instance_connections` | `instance_id`, `device_jid`, `status`, `last_connected_at`, `last_error_code`, `last_error_message`, `last_error_at` |
| `instance_webhooks` | `instance_id`, `url`, `is_enabled`, `events` |
| `users` | `email`, `password_hash`, `role`, `instance_limit` |
| `message_queue` | `instance_id`, `recipient_jid`, `message_type`, `payload`, `send_status`, `retry_count`, `wa_id`, `media_id`, `last_error_code`, `last_error_message`, `last_error_at`, `next_attempt_at`, `delivered_at`, `read_at` |
| `media` | `instance_id`, `direction`, `wa_id`, `mime_type`, `file_name`, `size_bytes`, `bucket`, `object_key`, `sha256`, `expires_at`, `object_deleted_at` |
| `jid_cache` | `phone`, `jid`, `expires_at` |
| `chatwoot_configs` | `instance_id`, `is_enabled`, `url`, `account_id`, `token`, `inbox_name`, `is_sign_enabled`, `sign_delimiter`, `is_reopen_enabled`, `is_pending_enabled`, `is_merge_enabled`, `is_import_contacts`, `is_import_messages`, `import_days`, `is_auto_create`, `organization`, `logo`, `ignored_jids` |
| `chatwoot_messages` | `instance_id`, `message_id`, `wa_key`, `cw_id`, `conversation_id`, `inbox_id`, `chat_jid`, `is_read` |
| `group_metadata` | `instance_id`, `group_jid`, `name`, `description`, `participant_count` |
| `channel_metadata` | `instance_id`, `channel_jid`, `title`, `description`, `follower_count` |
| `idempotency_keys` | `instance_id`, `key`, `request_hash`, `status`, `http_status`, `response_body`, `expires_at` |
| `event_outbox` | `subject`, `envelope`, `attempt_count`, `last_error_code`, `last_error_message`, `last_error_at` |
| `webhook_dead_letters` | `instance_id`, `event_id`, `event_type`, `envelope`, `attempt_count`, `last_error_code`, `last_error_message`, `last_error_at` |

Renomeações de tabelas: `contacts → jid_cache`; `newsletter_metadata → channel_metadata`. A divisão de `instances` acrescenta duas tabelas: 12 atuais tornam-se 14 próprias.

Renomeações de colunas relevantes: `instance_quota → instance_limit`; fila `recipient → recipient_jid`, `type → message_type`, `status → send_status`, `retries → retry_count`, `whatsapp_id → wa_id`; mídia `mimetype → mime_type`, `filename → file_name`; Chatwoot messages `chatwoot_message_id → cw_id`, `contact_source_id → chat_jid`; idempotência `idempotency_key → key`, `response_status → http_status`; outbox/dead letters `attempts → attempt_count`; dead letters `payload → envelope`. Para os flags Chatwoot, a lista curta acima é a final aprovada e substitui a proposta longa inicial.

### 3. Unicidade, cardinalidade e ownership

- `instance_connections.instance_id` e `instance_webhooks.instance_id`: FK + `UNIQUE`; criar identidade e seus dois registros juntos. Mover os campos, sem mantê-los duplicados em `instances`.
- `device_jid` exclusivo quando preenchido, incluindo o identificador completo do dispositivo. Sem pareamento pode ser `NULL`; retomada carrega somente o dispositivo vinculado. Novo pareamento explícito pode substituir o vínculo; ausência/rejeição de credenciais não autoriza trocar silenciosamente por outra sessão.
- `message_queue.media_id → media.id`: opcional, várias mensagens podem compartilhar uma mídia, mesma instância obrigatória. Expiração do objeto não rompe a FK.
- `chatwoot_messages.message_id → message_queue.id`: opcional e mesma instância. Entradas e edições sem registro na fila não recebem mensagens artificiais. Vários envios podem compartilhar `cw_id`; não criar `UNIQUE(cw_id)`.
- Manter `UNIQUE(instance_id, wa_key)`, `UNIQUE(instance_id, group_jid)`, `UNIQUE(instance_id, channel_jid)`, `UNIQUE(instance_id, key)` e `UNIQUE(instance_id)` para config Chatwoot.
- Manter `phone` único no cache global, `external_ref` único quando informado, email único sem diferenciar caixa, `event_id` único em dead letters e `UNIQUE(bucket, object_key)` para mídia.
- Nome da instância conserva o contrato existente de URL e lookup exato. Não impor uma nova unicidade SQL que torne migração de nomes legados ambíguos impossível sem uma estratégia explícita.
- O outbox não terá cascade de exclusão por instância: pendentes sobrevivem para concluir a publicação.

Substituir atualizações de snapshot inteiro de instância por comandos que alterem apenas identidade, conexão ou webhook. Isso evita que uma edição de nome regrave um estado de conexão antigo. As consultas por `wa_id` e as referências à fila/mídia devem considerar a instância.

Alternativas descartadas: FK obrigatória de toda correlação Chatwoot para a fila (inbound não está nela), mídia pertencente exclusivamente a uma mensagem (uploads podem preceder o envio e ser reutilizados), copiar estado/JID nas duas tabelas de instância.

### 4. Estados e erros

| Domínio | Valores aprovados |
|---|---|
| Conexão | `disconnected`, `pairing`, `connected`, `error` |
| Role | `admin`, `user` |
| Envio | `queued`, `sending`, `sent`, `failed` |
| Direção de mídia | `inbound`, `outbound` |
| Idempotência | `in_progress`, `completed` |

Validação no domínio e no banco. Não mudar o significado dos contadores: `retry_count` conta retentativas agendadas da fila; `attempt_count` do outbox conta publicações com falha; o de dead letters conta tentativas realizadas de entrega.

Erros ficam em três colunas nas quatro tabelas listadas. Erro ausente: três `NULL`; texto legado não vazio: `legacy_error`, texto original, instante desconhecido `NULL`. Para novos erros, capturar código e instante na origem, preservando causas tipadas; não extrair códigos de mensagens por heurística generalizada. Sucesso limpa o trio quando o domínio atual limpa seu erro; a falha definitiva de dead letter conserva o último erro da entrega.

O catálogo de códigos e o fallback de `NeedsFreshPairing` devem ser especificados na tarefa 1.1; conservar as strings dos eventos v1 e a lógica de transição existente.

### 5. JSON público aprovado

**BREAKING**: DTOs de transporte próprios, atualização conjunta de Go, Manager e Swagger. Não há equivalência automática entre nome SQL e JSON: SQL `is_enabled` vira `webhook.enabled` na instância; trio SQL de erro vira objeto `last_error`.

```json
{
  "data": {
    "items": [
      {
        "instance": {
          "id": "8abab8c9-b8bb-47c1-9b09-acd70e6c16fa",
          "name": "FELIPE",
          "connection": {
            "status": "disconnected",
            "last_error": {
              "code": "legacy_error",
              "message": "qr code expired",
              "occurred_at": null
            },
            "last_connected_at": "2026-10-04T02:57:26.241126Z"
          },
          "webhook": {
            "enabled": false,
            "url": null,
            "events": [
              "message",
              "receipt",
              "connection",
              "message.status"
            ]
          },
          "created_at": "2026-10-04T01:37:10.441826Z",
          "updated_at": "2026-10-04T03:19:52.039039Z"
        }
      }
    ]
  }
}
```

- Individuais: `data.instance`; coleções: `data.items[].instance`. Outras entidades usam suas próprias chaves (`message`, `user`, `group`, etc.). Arrays vazios são `[]`.
- Instância pública oculta `whatsapp_jid`, `device_jid`, `external_ref`, `owner_user_id` e hashes; referência externa e ownership continuam persistidos e aceitos pelos fluxos de escrita autorizados já existentes.
- Erros de conexão/envio: `{code, message, occurred_at}` ou `null`. Envelope HTTP de falha continua `{"error":{"code":"...","message":"..."}}`, com `X-Request-Id`.
- Senhas, hashes e tokens não saem em leitura. `instance_api_key` conserva o nome e é entregue somente nas respostas específicas de criação/rotação, dentro da representação correspondente; nenhuma leitura de instância passa a devolvê-la.
- Projeção de usuário fornece limite e contagem de uso pelo backend (`instances_used`, sem coluna de contador persistida); contar todas as instâncias possuídas, inclusive desconectadas e em erro. Preservar `0` ilimitado e bypass admin já existentes.
- Manager remove exposição/edição automática de `external_ref`, colunas de dono e JIDs públicos, mas conserva seleção autorizada de dono na criação quando cabível. Não enviar string vazia para apagar referências ocultas.
- Catálogo do webhook preserva opções existentes, inclusive eventos já configurados fora dos quatro defaults; o formulário não pode descartá-los ao salvar.
- Downloads, `204`, HTML/Swagger e confirmações externas, incluindo webhook Chatwoot, conservam seus formatos e status atuais. REST v1 do evento é distinto do DTO HTTP: NATS e webhooks mantêm `event_version: 1`, campos, strings de motivo/erro e ID estável.

O inventário inicial está em `response-matrix.md`, extraído do Swagger atual. Os contratos atuais não devem ser inferidos só pela tabela do banco; o destino final de cada rota e seus formatos de comandos/agregados serão preenchidos na tarefa 1.1 antes de implementar o corte.

### 6. MinIO e duração da mídia

- Imagem exata solicitada: `quay.io/minio/minio:RELEASE.2024-01-13T07-53-03Z-cpuv1`. A consulta retornou `no such manifest`; não há alternativa de tag autorizada.
- Compose de desenvolvimento e configuração de produção usam a rede existente do projeto e parâmetros por ambiente. O exemplo não autoriza Gacont, Traefik, domínios ou segredos fixos.
- Usar cliente S3/SDK Go; bytes no MinIO, metadados no PostgreSQL. `bucket` e `object_key` são identificadores estáveis, não URLs temporárias. Manter o download autenticado pela API e as URLs externas já usadas pelos eventos.
- Componentes que hoje pedem `Path` deverão consumir objetos por um contrato de leitura/materialização temporária com cleanup explícito, incluindo conclusão, erro e cancelamento. Bytes locais temporários não são a nova fonte persistente de mídia.
- Expiração torna o download indisponível; remover o objeto e só depois registrar `object_deleted_at`. Falha mantém a exclusão pendente para repetição; objeto já ausente permite concluir a limpeza de forma idempotente. Metadados e FK permanecem.
- Preservar UUID, SHA-256, tamanho e validade de mídia migrada. Copiar e verificar antes de trocar sua referência persistida. Registrar marcadores Chatwoot legados separadamente no relatório de migração, sem tratá-los como WA IDs. O descarte das origens precisa de estratégia de rollback/backup concluída.

Alternativa descartada: conservar o diretório local como armazenamento permanente depois do corte. Migração e rollback, porém, precisam de uma janela transitória explícita para os arquivos existentes.

### 7. Outbox, idempotência e Chatwoot

Outbox contém somente pendentes. Publicar com o mesmo `event_id`; excluir do SQL apenas após PubAck do JetStream. Falha ou confirmação incerta deixa o evento pendente. Ack seguido de falha na exclusão pode republicar; consumidores deduplicam por evento. Não prometer exactly-once nem que PubAck significa processamento por todos os consumidores. JetStream conserva a retenção própria. `published_at` é eliminado; registros legados confirmados podem ser retirados durante a migração, sem apagar pendentes.

Idempotência mantém seu escopo, fingerprint, expiração, status HTTP e autorização antes do replay. Converter respostas legadas por contrato/rota sem reenviar mensagens, sem invalidar todo o cache e sem reproduzir campos públicos removidos. Se houver resposta legada não conversível, a tarefa 1.1 deve definir uma resposta segura que não execute a operação novamente; não inventar essa política no implementador.

Correlações Chatwoot referenciam UUID de fila para saídas, preservam inbound/edições e `UNIQUE(instance_id, wa_key)`. Uma saída existe antes de receber WA ID: a representação de correlação nessa janela precisa ser fechada na tarefa 1.1, sem usar chaves sintéticas como IDs reais nos comandos de leitura/revoke/quoted. Conservar idempotência de mirror/import e todas as flags atuais, incluindo proteção opcional do token em repouso.

## Risks / Trade-offs

- [Tag MinIO indisponível] → manter a exigência documentada e obter imagem exata verificável antes do teste/deploy; não retaggear uma imagem diferente para parecer compatível.
- [FKs novas encontram órfãos de mídia já apagada] → auditoria antes do DDL; não inventar arquivos/hash/metadados nem apagar silenciosamente o UUID histórico; fechar política de reparo na tarefa 1.1.
- [Valores legados inválidos ou nomes ambíguos] → preflight de estados, enums, propriedade, JIDs e duplicatas; preservar lookup UUID/nome existente e parar diante de conflitos não resolvidos.
- [Perda de atualização de conexão] → comandos separados e teste de edição de identidade concorrente com transição de estado.
- [JSON antigo reaparece por replay] → converter cache sem reexecutar efeitos e testar ocultação dos campos, incluindo respostas antigas de criação.
- [MinIO apaga bytes antes do registro SQL] → cleanup idempotente e reconhecimento de objeto ausente; índices para limpar apenas objetos expirados ainda não confirmados.
- [Contrato de eventos muda por reutilizar DTOs] → serialização de evento e HTTP independentes com testes de fixtures v1.
- [Timestamp de erro inventado] → `NULL` para legado e relógio na origem de novas falhas.
- [Alterações locais de outra change são absorvidas] → coordenar baseline/migração 00007 e preservar o diff existente; futuro apply em `.worktrees/`.

## Migration Plan

1. Concluir tarefa 1.1 e os bloqueios de execução listados abaixo, com matriz de schemas/rotas e fixtures.
2. Coordenar a base Git e migração 00007; criar backup/relatório de preflight, ensaiar banco vazio e upgrade com fixtures isoladas.
3. Preparar MinIO exato e transferência verificável dos objetos, mantendo as origens recuperáveis durante o ensaio.
4. Aplicar expansão/backfill e validação de vínculos conforme a estratégia final; qualquer conflito preserva os dados e bloqueia o corte.
5. Atualizar serviços/repositórios, transportes, Manager e Swagger; converter cache de idempotência e exercer eventos v1.
6. Fazer o corte coordenado com pausa de writers quando necessária; conferir IDs, contagens, ownership, quotas, relações e objetos.
7. Remover colunas antigas apenas após a validação. Rollback de SQL/arquivos/respostas/cache deve ser definido e ensaiado antes do corte; não prometer rollback automático de UUIDs novos para bigint.

## Decisões fechadas na task 1.1

Estas decisões fecham os cinco bloqueios pendentes do pedido aprovado. Nenhuma decisão material permanece aberta para o implementador da etapa 2.

### 1. MinIO: imagem, bucket, endpoint, credenciais e provisionamento

**Imagem**: `docker.io/cccs/minio` fixada por digest do manifest — `docker.io/cccs/minio@sha256:68eefa6a5ccd82178a872b2d1012687d0ac9b1afa848a1e56f55fa5f1efcb081` (RELEASE.2024-12-18T13-15-44Z, AGPL, publicado pelo Canadian Centre for Cyber Security; digest verificado no cache local e por GET autenticado ao registry no preflight de 2026-10-06, e o binário serve S3 e responde `/minio/health/live`). A referência registrada nesta decisão era `:latest`; a task 6.1 da revisão final trocou todas as ocorrências do compose (`docker-compose.yml` e `docker-compose.dev.yml`) pelo pin por digest, que é imutável e reprodutível — `:latest` continua resolvendo para o mesmo manifest hoje, mas o pin elimina o risco de deriva futura. A imagem substitui a tag originalmente pedida `quay.io/minio/minio:RELEASE.2024-01-13T07-53-03Z-cpuv1`: o quay.io tornou-se inacessível para o fluxo de verificação deste projeto a partir de mai/2025 (a MinIO removeu o repositório do Docker Hub e trancou o quay atrás de login), inviabilizando verificar tags oficiais sem credenciais. A tag `cccs` é a última linha pública anterior à restrição e é mantida por um fornecedor governamental. Risco residual: fornecedor não-oficial; mitigado por ser imagem governamental que serve MinIO real verificado em runtime e pelo pin digest que congela exatamente os bytes verificados.

**Configuração final**:

| Parâmetro | Valor |
|---|---|
| Bucket | `wzap-media` |
| Endpoint (dev/compose) | `http://minio:9000` na rede `wzap` existente do compose |
| Região | `us-east-1` (assinatura S3 v4; sem significado real no MinIO) |
| TLS | off em dev/compose; configurável por env em produção |
| Porta console | `9001` (não exposta em dev) |

**Credenciais** (padrão `WZAP_*` de `internal/config/config.go`):
- `WZAP_S3_ENDPOINT` — URL do endpoint (default dev `http://minio:9000`)
- `WZAP_S3_BUCKET` — nome do bucket (default `wzap-media`)
- `WZAP_S3_REGION` — região de assinatura (default `us-east-1`)
- `WZAP_S3_ACCESS_KEY` / `WZAP_S3_SECRET_KEY` — credenciais do serviço
- `WZAP_S3_USE_TLS` — bool, default `false` em dev

O MinIO do compose aceita `MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD` próprios do serviço; o app só conhece os `WZAP_S3_*`, nunca os `MINIO_*` diretamente.

**Provisionamento do bucket**: init container no compose (`minio/mc` executando `mb -p wzap-media` após aguardar o healthcheck) e, em produção, passo de bootstrap do app na subida: o serviço `media` chama `EnsureBucket(ctx)` no startup, criando o bucket se ausente e tratando `BucketAlreadyOwnedByYou` como sucesso. Escolha: bootstrap do app (não migração goose) porque o bucket não é schema PostgreSQL; o init container no compose apenas acelera o caminho feliz em dev, sem ser contrato. Justificativa: mantém a regra de que falha de provisionamento em produção deve impedir o armazenamento de mídia sem depender de orquestração externa.

### 2. Matriz final de schema das 14 tabelas

Convenções aplicadas a todas as tabelas próprias: `id uuid PRIMARY KEY` (as tabelas hoje com PK natural/composta recebem `id` novo via `gen_random_uuid()` e sua combinação original vira `UNIQUE`), `created_at timestamptz NOT NULL DEFAULT now()`, `updated_at timestamptz NOT NULL DEFAULT now()`. Tipos base herdados conforme decisão 1 do design: texto para JIDs/chaves/telefones/hashes, `jsonb` para payloads/envelopes, `text[]` para listas, `int`/`bigint` para contadores e IDs Chatwoot.

**Enums SQL**: `CHECK` constraints inline (não `CREATE TYPE`). Justificativa: os domínios são pequenos, estáveis e fechados (4–5 valores); um `CREATE TYPE` implicaria migração destrutiva para acrescentar valor, além de acoplamento entre migrações. CHECK mantém o valor legível no dump, permite evoluir com `ALTER TABLE ... DROP CONSTRAINT/ADD CONSTRAINT` sem recriar a tabela e é a convenção já usada em `users.role` (`00002_product.sql`). Os valores válidos são os aprovados na decisão 4: conexão (`disconnected`,`pairing`,`connected`,`error`), role (`admin`,`user`), envio (`queued`,`sending`,`sent`,`failed`), direção de mídia (`inbound`,`outbound`), idempotência (`in_progress`,`completed`).

**`updated_at`**: atualizado pela aplicação (repositório escreve `updated_at = now()` em todo UPDATE/UPSERT), não por trigger. Justificativa: o projeto não usa triggers hoje; o `postgrestest` e os testes de repositório observam `updated_at` via aplicação; um trigger acrescentaria estado implícito ao schema e quebraria a simetria dos fakes em `internal/storage/postgres/`.

#### 2.1 `instances`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| name | text | NOT NULL | — | — |
| external_ref | text | NULL | — | UNIQUE |
| owner_user_id | uuid | NULL | — | FK → users.id ON DELETE SET NULL |
| api_key_hash | text | NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

Índices: `instances_device_jid` sai daqui (vai para `instance_connections`). `instances_external_ref_uidx` UNIQUE já coberto pela constraint. `instances_owner_idx` em `(owner_user_id)` para `CountByOwner`/filtros de sessão. `instances_name_idx` em `(name)` para o lookup exato por nome (sem UNIQUE: nomes legados ambíguos existem e a política é tratamento explícito, não bloqueio SQL).

#### 2.2 `instance_connections`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| instance_id | uuid | NOT NULL | — | UNIQUE, FK → instances.id ON DELETE CASCADE |
| device_jid | text | NULL | — | UNIQUE WHERE not null/empty (índice parcial, preserva `instances_device_jid_uidx` de 00007) |
| status | text | NOT NULL | 'disconnected' | CHECK IN (disconnected,pairing,connected,error) |
| last_connected_at | timestamptz | NULL | — | — |
| last_error_code | text | NULL | — | — |
| last_error_message | text | NULL | — | — |
| last_error_at | timestamptz | NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

Observação: `whatsapp_jid` legado migra para `device_jid` quando ausente (regra de 00007); divergências documentadas no relatório de migração.

#### 2.3 `instance_webhooks`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| instance_id | uuid | NOT NULL | — | UNIQUE, FK → instances.id ON DELETE CASCADE |
| url | text | NULL | — | — |
| is_enabled | bool | NOT NULL | false | — |
| events | text[] | NOT NULL | '{message,receipt,connection,message.status}' | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

#### 2.4 `users`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| email | text | NOT NULL | — | UNIQUE via `users_email_lower_idx` em lower(email) |
| password_hash | text | NOT NULL | — | — |
| role | text | NOT NULL | — | CHECK IN (admin,user) |
| instance_limit | int | NOT NULL | 0 | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

**Política de remoção de usuário**: `instances.owner_user_id` é `ON DELETE SET NULL` — remover uma conta preserva suas instâncias (elas passam a ter dono `NULL`, como os registros legados pré-backfill). A remoção de usuário **não** apaga instâncias: o handler `DELETE /users/{id}` já rejeita contas com instâncias hoje (verificação de `CountByOwner > 0` antes do delete, comportamento preservado); a FK com SET NULL é a rede de segurança para o caso de bypass futuro e para instâncias órfãs legadas que já têm `owner_user_id NULL`. Justificativa: as instâncias são recursos operacionais independentes do operador; apagá-las em cascata destruiria sessões e filas.

#### 2.5 `message_queue`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| recipient_jid | text | NOT NULL | — | — |
| message_type | text | NOT NULL | — | — |
| payload | jsonb | NOT NULL | '{}' | — |
| send_status | text | NOT NULL | 'queued' | CHECK IN (queued,sending,sent,failed) |
| retry_count | int | NOT NULL | 0 | — |
| wa_id | text | NULL | — | — |
| media_id | uuid | NULL | — | FK → media.id ON DELETE SET NULL |
| last_error_code | text | NULL | — | — |
| last_error_message | text | NULL | — | — |
| last_error_at | timestamptz | NULL | — | — |
| next_attempt_at | timestamptz | NULL | — | — |
| delivered_at | timestamptz | NULL | — | — |
| read_at | timestamptz | NULL | — | — |

`media_id` ON DELETE SET NULL: uma mídia expirada/removida não pode derrubar a mensagem que a referenciava. A regra "mesma instância" é garantida no repositório (a aplicação sempre consulta `media.id` junto com `instance_id`), sem triggers — uma `CHECK` declarativa não cobre validação cross-table. Política: validação no repositório (SELECT media WHERE id=$1 AND instance_id=$2) mais assert de integridade no pós-migração. Índices: `message_queue_instance_status_idx (instance_id, send_status)`, `message_queue_created_idx (created_at)`, `message_queue_wa_id_idx (wa_id)`, `message_queue_media_idx (media_id) WHERE media_id IS NOT NULL`, `message_queue_next_attempt_idx (next_attempt_at) WHERE send_status='queued'` (suporta `ClaimQueued`).

#### 2.6 `media`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| direction | text | NOT NULL | — | CHECK IN (inbound,outbound) |
| wa_id | text | NULL | — | — |
| mime_type | text | NOT NULL | — | — |
| file_name | text | NULL | — | — |
| size_bytes | bigint | NOT NULL | — | — |
| bucket | text | NOT NULL | — | — |
| object_key | text | NOT NULL | — | — |
| sha256 | text | NOT NULL | — | — |
| expires_at | timestamptz | NOT NULL | — | — |
| object_deleted_at | timestamptz | NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

`UNIQUE(bucket, object_key)`. `wa_id` recebe o `message_id` legado somente quando ele não é marcador `chatwoot-*` (ver política não destrutiva abaixo). Índices: `media_expires_idx (expires_at) WHERE object_deleted_at IS NULL`, `media_instance_idx (instance_id)`, `media_wa_id_idx (wa_id) WHERE wa_id IS NOT NULL`.

#### 2.7 `jid_cache`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| phone | text | NOT NULL | — | UNIQUE |
| jid | text | NOT NULL | — | — |
| expires_at | timestamptz | NOT NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

Índices: `jid_cache_expires_idx (expires_at)`, `jid_cache_phone_idx (phone)` (o UNIQUE já cobre; índice explícito não necessário — manter apenas o UNIQUE). Renomeada de `contacts`; `phone` deixa de ser PK e passa a UNIQUE.

#### 2.8 `chatwoot_configs`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| instance_id | uuid | NOT NULL | — | UNIQUE, FK → instances.id ON DELETE CASCADE |
| is_enabled | bool | NOT NULL | false | — |
| url | text | NOT NULL | '' | — |
| account_id | text | NOT NULL | '' | — |
| token | text | NOT NULL | '' | — |
| inbox_name | text | NOT NULL | '' | — |
| is_sign_enabled | bool | NOT NULL | false | — |
| sign_delimiter | text | NOT NULL | '' | — |
| is_reopen_enabled | bool | NOT NULL | true | — |
| is_pending_enabled | bool | NOT NULL | false | — |
| is_merge_enabled | bool | NOT NULL | false | — |
| is_import_contacts | bool | NOT NULL | false | — |
| is_import_messages | bool | NOT NULL | false | — |
| import_days | int | NOT NULL | 0 | — |
| is_auto_create | bool | NOT NULL | false | — |
| organization | text | NOT NULL | '' | — |
| logo | text | NOT NULL | '' | — |
| ignored_jids | text[] | NOT NULL | '{}' | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

Renomeações: `enabled→is_enabled`, `name_inbox→inbox_name`, `sign_msg→is_sign_enabled`, `reopen_conversation→is_reopen_enabled`, `conversation_pending→is_pending_enabled`, `merge_brazil_contacts→is_merge_enabled`, `import_contacts→is_import_contacts`, `import_messages→is_import_messages`, `days_limit→import_days`, `auto_create→is_auto_create`, `ignore_jids→ignored_jids`.

#### 2.9 `chatwoot_messages`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK (novo) |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| message_id | uuid | NULL | — | FK → message_queue.id ON DELETE SET NULL |
| wa_key | text | NOT NULL | — | — |
| cw_id | bigint | NOT NULL | — | — |
| conversation_id | bigint | NOT NULL | — | — |
| inbox_id | bigint | NOT NULL | — | — |
| chat_jid | text | NOT NULL | '' | — |
| is_read | bool | NOT NULL | false | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

`UNIQUE(instance_id, wa_key)` preservada (era a PK composta). `UNIQUE(instance_id, cw_id)` **não** criada — vários envios podem compartilhar o mesmo `cw_id` (múltiplos anexos). `message_id` SET NULL: apagar a fila não remove a correlação. Índices: `chatwoot_messages_instance_msg_idx (instance_id, cw_id)` (renomear da antiga `chatwoot_message_id`), `chatwoot_messages_conversation_idx (instance_id, conversation_id, created_at DESC, cw_id DESC)` (evolui o índice de 00004), `chatwoot_messages_message_id_idx (message_id) WHERE message_id IS NOT NULL`.

#### 2.10 `group_metadata`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK (novo) |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| group_jid | text | NOT NULL | — | — |
| name | text | NOT NULL | '' | — |
| description | text | NOT NULL | '' | — |
| participant_count | int | NOT NULL | 0 | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

`UNIQUE(instance_id, group_jid)` (era PK composta).

#### 2.11 `channel_metadata` (renomeada de `newsletter_metadata`)

Mesma forma de `group_metadata`: `id` novo, `UNIQUE(instance_id, channel_jid)`, colunas `title`, `description`, `follower_count`.

#### 2.12 `idempotency_keys`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK (novo) |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| key | text | NOT NULL | — | — |
| request_hash | text | NOT NULL | — | — |
| status | text | NOT NULL | — | CHECK IN (in_progress,completed) |
| http_status | int | NULL | — | — |
| response_body | jsonb | NULL | — | — |
| expires_at | timestamptz | NOT NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

`UNIQUE(instance_id, key)` (era PK composta). Índice `idempotency_keys_expires_idx (expires_at)` preservado.

#### 2.13 `event_outbox`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK |
| subject | text | NOT NULL | — | — |
| envelope | jsonb | NOT NULL | — | — |
| attempt_count | int | NOT NULL | 0 | — |
| last_error_code | text | NULL | — | — |
| last_error_message | text | NULL | — | — |
| last_error_at | timestamptz | NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

`published_at` eliminado: registros confirmados são removidos no corte; a tabela só guarda pendentes. Índice `event_outbox_pending_idx (created_at)` (não precisa mais de filtro `published_at IS NULL` — tudo é pendente).

#### 2.14 `webhook_dead_letters`

| coluna | tipo | NULL | default | constraints |
|---|---|---|---|---|
| id | uuid | NOT NULL | gen_random_uuid() | PK (novo, substitui bigserial) |
| instance_id | uuid | NOT NULL | — | FK → instances.id ON DELETE CASCADE |
| event_id | uuid | NOT NULL | — | UNIQUE |
| event_type | text | NOT NULL | '' | — |
| envelope | jsonb | NOT NULL | '{}' | — |
| attempt_count | int | NOT NULL | 0 | — |
| last_error_code | text | NULL | — | — |
| last_error_message | text | NULL | — | — |
| last_error_at | timestamptz | NULL | — | — |
| created_at / updated_at | timestamptz | NOT NULL | now() | — |

Mapeamento do `id` bigint antigo: ver política não destrutiva abaixo. Índice `webhook_dead_letters_instance_idx (instance_id, created_at DESC, id DESC)` preservado (ordenação por `created_at`+`id` uuid agora).

**Resumo das ações ON DELETE**: CASCADE para tudo que pertence estritamente à instância (conexão, webhook, fila, mídia, config/correlações chatwoot, metadados, idempotência, dead letters, cache JID não tem FK). SET NULL para `instances.owner_user_id` e `chatwoot_messages.message_id` e `message_queue.media_id`. `event_outbox` não tem FK por instância (pendentes sobrevivem à exclusão da instância — decisão já registrada).

### 3. Política não destrutiva

**Preflight obrigatório antes do DDL** (executado no banco origem, saída em relatório):
- auditoria de `message_queue.media_id` → `media.id` inexistente;
- auditoria de `instances.whatsapp_jid` vs `device_jid` divergentes (campos legados);
- auditoria de `chatwoot_messages` sem `wa_key` válida;
- contagem de duplicatas de `instances.name` e `external_ref`;
- verificação de estados fora dos enums.

**Mídia órfã** (`message_queue.media_id` aponta para `media.id` inexistente): a linha de `message_queue` é preservada e seu `media_id` é anulado durante a migração, com a linha registrada no relatório (`orphan_media_refs` listando `message_queue.id`, `instance_id`, `media_id` antigo). Nenhum arquivo/checksum/metadado fictício é criado. A FK nova só é criada após o cleanup, então o corte não falha por órfãos. Justificativa: o objeto já se foi (limpeza anterior ou falha); anular a referência preserva a mensagem e seus dados de envio — o que era recuperável já não estava acessível.

**Mídia com marcador `chatwoot-*` em `message_id` legado**: o `media.message_id` atual mistura WA IDs reais e marcadores `chatwoot-{msgID}-{idx}` gerados pelo inbound. Na migração para `media.wa_id`: valores que satisfaçam o padrão `chatwoot-\d+-\d+` **não** são copiados para `wa_id` (ficam `NULL`); são registrados no relatório de migração (`media_chatwoot_markers` com `media.id`, marcador original). Nenhum marcador sintético pode circular como `wa_id` — leitura/reply/revoke dependem de `wa_id` real.

**JIDs divergentes em `jid_cache`**: `phone` é UNIQUE no novo schema; se o preflight encontrar duplicatas de `phone` (impossível hoje, PK natural — mas defensivo) ou `jid` vazio/malformado, a linha é preservada, o `phone` afetado é registrado no relatório e a constraint UNIQUE só é aplicada após resolução manual. `instances.whatsapp_jid` vs `device_jid` divergentes: ambos são copiados para `instance_connections.device_jid` preferindo `device_jid` (regra de 00007); divergências entram no relatório (`device_jid_conflicts`) e bloqueiam o corte até resolução explícita — nunca escolher silenciosamente.

**Mapeamento bigint→UUID em `webhook_dead_letters`**: nova coluna `id uuid DEFAULT gen_random_uuid()`; o bigint antigo é descartado **após** confirmar que `event_id` (o identificador estável, já UNIQUE) permanece idêntico. Consumidores deduplicam por `event_id`, então o `id` interno bigint não tem significado externo — a substituição é segura. `attempt_count` e `envelope` (ex-`payload`) preservados; `last_error` → `last_error_code='legacy_error'`, `last_error_message=texto`, `last_error_at=NULL`.

**Rollback/backup**: antes de qualquer DDL de corte, `pg_dump` completo do schema `public` (14 tabelas + WhatsMeow) + dump apenas-dados das 14 tabelas próprias para restauração seletiva; cópia do diretório `WZAP_DATA_DIR` de mídia para o volume de staging. Rollback: restaurar o dump em banco paralelo, repontar o app, validar contagens/UUIDs; não há downgrade de schema in-app (o DDL é expansão+renomeação com colunas novas; o corte remove as antigas só após validação). Para mídia: arquivos locais permanecem intocados durante a janela de migração MinIO — só são removidos depois de confirmada a cópia+checksum e após o corte SQL validado.

### 4. Correlação Chatwoot pré-WA ID

**Janela**: `POST /instances/{id}/messages*` responde 202 com `data.message.id` (UUID da fila) no ato do enfileiramento; `wa_id` só chega depois, via MarkSent. Nessa janela `chatwoot_messages` precisa registrar a correlação de uma saída que ainda não tem `wa_key` real.

**Regra fechada**: o webhook Chatwoot (`inbound/webhook.go`) passa a capturar o UUID retornado por `Enqueue` (hoje descartado nas chamadas `_, err := h.enqueuer.Enqueue(...)`, linhas ~388, ~407, ~427). Para cada envio enfileirado a serviço de uma mensagem Chatwoot `cw_id`, é criada/atualizada uma linha em `chatwoot_messages` com `message_id = <uuid da fila>`, `cw_id = <id da mensagem Chatwoot>`, `conversation_id`/`inbox_id` do payload e `wa_key` **sintética provisória** `pending:{uuid}` — prefixo reservado que nunca é um WA ID real e nunca é usado como argumento em comandos de leitura, reply ou revoke.

Quando o `MarkSent` da fila grava o `wa_id` real, o worker de mirror (que consome os eventos de saída) atualiza a linha: `wa_key = wa_id real`. A `UNIQUE(instance_id, wa_key)` é preservada: duas saídas de uma mesma mensagem Chatwoot têm `pending:{uuid1}` e `pending:{uuid2}` distintos, e depois `wa_id` distintos.

**Múltiplos anexos**: uma mensagem Chatwoot com N anexos gera N `Enqueue` (um por anexo que baixou com sucesso), cada um com sua própria linha `chatwoot_messages` — `message_id` distinto, mesmo `cw_id`. Por isso `UNIQUE(instance_id, cw_id)` foi explicitamente rejeitada: a cardinalidade é N:1. Isolamento por instância garantido por `instance_id` em todas as buscas.

**Falhas parciais**: o loop de anexos já é best-effort hoje (falha em download/armazenamento/enqueue → nota privada + `continue`). A política fecha: cada anexo enfileirado com sucesso recebe sua correlação; anexos que falharam não recebem linha — não há placeholder. Se zero anexos enfileirarem e houver texto, o fallback de texto já existente corre; sua correlação segue a mesma regra.

**Backfill**: correlações legadas (pré-mudança) têm `wa_key` real e `message_id NULL`. Backfill de `message_id` **somente com evidência**: junção por `wa_key = message_queue.wa_id` e `instance_id` iguais. Nenhuma heurística por tempo/destinatário. Correlações inbound e de edições (`wa_key` de edição, sem linha na fila) permanecem com `message_id NULL` — nunca recebem mensagem artificial na fila.

**Atomicidade**: `Enqueue` + `INSERT chatwoot_messages` são duas escritas; não são transação única (fila e correlação em repositórios distintos). A ordem é: Enqueue primeiro, depois PUT da correlação com o UUID retornado — se o PUT falhar, a mensagem existe na fila sem correlação (recuperável pelo backfill com evidência assim que `wa_id` chegar); nunca correlação antes do enqueue (não existe UUID ainda).

### 5. Catálogo de erros e idempotência legada

**Códigos de `last_error_code`** (domínio fechado, CHECK não aplicado — texto livre com catálogo de domínio, pois códigos novos surgem na fonte sem migração de schema):

| código | quando | tabelas |
|---|---|---|
| `legacy_error` | texto legado migrado sem código estruturado | as 4 |
| `session_rejected` | `session.SessionRejectedReason` gravado em conexão | instance_connections |
| `logged_out` | last_error contém "logged out:" | instance_connections |
| `stream_replaced` | "stream replaced" | instance_connections |
| `device_jid_mismatch` | "device jid mismatch" | instance_connections |
| `device_jid_taken` | "device jid already bound" / `ErrDeviceJIDTaken` | instance_connections |
| `send_failed` | falha de envio ao upstream (MarkFailed) | message_queue |
| `send_retry` | retentativa agendada (MarkRetrying) | message_queue |
| `publish_failed` | `MarkAttempt` do outbox | event_outbox |
| `delivery_failed` | tentativa de webhook esgotada | webhook_dead_letters |
| `upstream_error` | erro tipado do whatsmeow sem código próprio | as 4 |

**Fallback `NeedsFreshPairing`**: hoje `session.NeedsFreshPairing(lastError)` testa substrings do texto (`pairing.go`). Com o código estruturado, a verificação passa a preferir `last_error_code IN ('session_rejected','logged_out','stream_replaced','device_jid_mismatch','device_jid_taken')`; quando o código é `legacy_error` ou `NULL`, mantém o teste de substring sobre `last_error_message` como fallback — as strings v1 existentes continuam sendo reconhecidas. Assim a transição legado→novo não quebra a recuperação de pareamento. Os eventos v1 conservam as strings de motivo/erro já publicadas (contrato separado dos DTOs HTTP).

**Resposta legada de idempotência não conversível**: uma resposta armazenada em `idempotency_keys.response_body` que não pode ser convertida ao contrato novo (ex.: corpo de operação cuja forma mudou sem mapeamento, ou corpo truncado/inválido) responde **410 Gone** com `{"error":{"code":"idempotency_response_expired","message":"cached response predates the current contract; retry without the idempotency key or with a new one"}}`. Nunca reexecuta o efeito (sem novo envio), nunca devolve o corpo cru legado (campos removidos poderiam reaparecer), nunca invalida a chave para reaproveitamento automático (o fingerprint já validou — o caller decide conscientemente). Justificativa: a alternativa de "solta a chave e reprocessa" enviaria a mensagem duas vezes; a alternativa de servir o corpo cru violaria o contrato público. 410 é semântico: o recurso existiu e não existe mais naquela representação. Status `in_progress` legado nunca é não-conversível (não há corpo).
