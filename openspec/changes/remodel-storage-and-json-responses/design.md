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

## Bloqueios para fechar o plano de execução

Estes pontos são explicitamente pendentes no pedido aprovado. A tarefa 1.1 deve resolvê-los antes da tarefa 2.1; não são decisões delegadas silenciosamente ao implementador.

1. Origem verificável da imagem exata MinIO e configuração final de bucket, endpoint, região/TLS, credenciais e provisionamento.
2. Matriz final de tipos, nullabilidade/defaults/enums, política de `updated_at`, índices e ações de exclusão. Incluir preservação de owners legados e remoção de usuário com instâncias.
3. Política não destrutiva para mídia órfã e JIDs divergentes, mapeamento dos IDs bigint antigos de dead letters e rollback/backup.
4. Representação e atomicidade da correlação Chatwoot antes do WA ID, múltiplos anexos e falhas parciais; backfill só com evidência de vínculo.
5. Destino JSON exato de todas as rotas em `response-matrix.md`, entradas afetadas, catálogo de erros e conversão dos caches legados; corpo não conversível não pode provocar novo envio.

O registro documental fica completo com esses bloqueios visíveis. A indicação de artefatos completos no OpenSpec não substitui o fechamento deles.
