# Decisões aprovadas — 2026-10-06

## Origem e ordem da discussão

O pedido começou com uma revisão dos responses REST e das tabelas, usando o schema PostgreSQL da Evolution como referência. O usuário priorizou o banco e aprovou os nomes tabela por tabela antes de pedir o registro junto da estrutura JSON inicialmente discutida.

Referência consultada: https://github.com/evolution-foundation/evolution-api/blob/main/prisma/postgresql-schema.prisma. Aproveitamos separação de responsabilidades e nomes explícitos, mantendo a fila de saída existente e o armazenamento de credenciais próprio do WhatsMeow.

## Padrões globais

- Todas as tabelas próprias: `id` UUID, `created_at`, `updated_at`.
- Inglês com `snake_case`; referência por entidade e papel (`instance_id`, `owner_user_id`); booleanos `is_`/`has_`; contadores `_count`.
- IDs externos, números, JIDs e chaves naturais mantêm significado e tipo adequados; antigos PKs naturais/compostos tornam-se `UNIQUE`.
- Tabelas do WhatsMeow e do Goose excluídas da remodelagem.

## Instâncias

Separação aprovada em identidade/acesso, conexão e webhook. `status` e `device_jid` ficam somente em `instance_connections`, sem cópias em `instances`. Dispositivo vinculado exclusivo; reconexão carrega as credenciais desse dispositivo. O fallback legado `whatsapp_jid` é removido após conferir a migração.

`external_ref` foi mantido com esse nome; referência e dono continuam no banco, embora ocultos das respostas públicas. `name` permanece como nome e endereço URL já suportado pelo sistema.

## Mensagens, mídia e Chatwoot

Fila continua apenas de saída e conserva estado atual: não foi aprovado histórico de mudanças nem um repositório geral de mensagens inbound. Nome externo da mensagem abreviado de `whatsapp_message_id` para `wa_message_id` e depois para o nome final aprovado `wa_id`.

Mensagem referencia mídia (`message_queue.media_id → media.id`), de forma opcional e dentro da mesma instância; o mesmo arquivo pode servir a vários envios. Expiração elimina os bytes, mantendo o registro e as referências.

Correlação Chatwoot tem `message_id` UUID opcional para a fila. Inbound e edições continuam independentes dessa fila; uma mensagem Chatwoot com múltiplos anexos pode originar vários envios. `wa_key` permanece, pois também comporta chaves de edição. Nomes longos da config foram rejeitados; a lista curta em `design.md` é a final aprovada.

Na inspeção constatamos que `media.message_id` legado também contém marcadores `chatwoot-...`. Eles não são IDs de mensagens WhatsApp e não podem ser copiados como tal para `media.wa_id`.

## WhatsMeow e cache

Foi aprovada `jid_cache` no lugar de `contacts`. Ela guarda resultado de resolução telefone → JID com validade; não duplica nomes/agenda nem a tabela de mapeamento de LIDs. ContactStore e LIDStore da versão fixada da biblioteca devem ser utilizados através do adapter; ausência de contato não prova ausência de conta WhatsApp.

## MinIO

Imagem exigida: `quay.io/minio/minio:RELEASE.2024-01-13T07-53-03Z-cpuv1`. Compose e produção usarão a rede já existente do projeto. O bloco Swarm recebido foi referência, não autorização para copiar rede Gacont, Traefik, domínios ou credenciais.

`bucket` e `object_key` substituem `storage_path`; `object_deleted_at` permite limpar o objeto sem apagar seus metadados. A consulta ao Quay retornou `no such manifest`; nenhuma tag alternativa foi autorizada.

## Eventos

O usuário questionou o armazenamento de eventos e escolheu manter só pendentes no PostgreSQL. Remover após confirmação de publicação pelo JetStream; falhas de publicação preservam os registros. `published_at` é eliminado. Retenção dos publicados cabe ao JetStream; IDs permanecem estáveis.

Última tabela aprovada: `webhook_dead_letters`, com UUID próprio, envelope, erro estruturado, contagem e limite de 500 falhas por instância.

## JSON

Tudo que compõe a representação da instância fica dentro de `instance`, com `connection` e `webhook` aninhados. Coleções usam `data.items[]` com um objeto do recurso em cada item. Remover JIDs, proprietário e referência externa dos DTOs públicos de instância; eventos v1 conservam seu formato.

Erros públicos de conexão/envio: `{code, message, occurred_at}` ou `null`. No SQL ficam em três colunas. O exemplo exato e a lista das 14 tabelas estão em `design.md`.

## O que este pedido autoriza agora

O plano solicitado é o registro dessas decisões em uma change OpenSpec, com proposta, specs, design, tarefas e plano. Tipos, obrigatoriedade, exclusão, vínculos legados e matriz por rota deverão ser concluídos antes da futura refatoração. Nenhuma migração, alteração do runtime, implantação ou descarte de arquivos foi executado neste registro.
