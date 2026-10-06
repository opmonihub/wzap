# Inventário das respostas por rota

Fonte: `docs/swagger.json` do workspace em 2026-10-06. São 89 operações documentadas em 72 paths. Inventário confrontado com o registro de rotas em `internal/httpapi/server.go`; nenhuma rota autenticada faltou no Swagger, e as diferenças handler↔Swagger estão registradas na coluna de destino.

## Regras fechadas para o DTO final (task 1.1)

1. **Recursos persistidos**: leitura/criação/atualização de entidade vai em `data.<entidade>`; coleções em `data.items[].<entidade>`, com `data.next_cursor` onde a página existe hoje. Arrays vazios são `[]`, nunca `null`. Chaves de entidade: `instance`, `connection`, `message`, `user`, `group`, `channel`, `media`, `chatwoot_config`, `me`, `stats`.
2. **Comandos e leituras derivadas sem entidade persistida**: o objeto de resultado vai direto em `data` com os campos atuais preservados (ex.: `data.pairing_code`, `data.link`, `data.exists`). Renomeação só se aplica onde o campo toca o modelo remodelado (status/last_error/whatsapp_jid de conexão e os campos de mensagem).
3. **`instance` pública**: `{id, name, connection:{status, last_error:{code,message,occurred_at}|null, last_connected_at|null}, webhook:{enabled, url|null, events[]}, created_at, updated_at}`. Oculta `whatsapp_jid`, `device_jid`, `external_ref`, `owner_user_id`, `api_key_hash`. `instance_api_key` só em create e rotate.
4. **`message` pública**: `{id, instance_id, message_type, recipient_jid, send_status, wa_id|null, media_id|null, retry_count, last_error:{code,message,occurred_at}|null, next_attempt_at|null, delivered_at|null, read_at|null, created_at, updated_at}`. Envios aceitos (202) devolvem `data.message` com `id` + `send_status:"queued"` (substitui o antigo `{message_id, status}`); `wa_id` só aparece quando o upstream confirma.
5. **`user` pública**: `{id, email, role, instance_limit, instances_used, created_at, updated_at}`. `instances_used` é contagem do backend (todas as instâncias do dono, qualquer estado); `instance_limit` substitui `instance_quota` (0 = ilimitado preservado). Senha/hash nunca.
6. **`chatwoot_config` pública**: `{instance_id, is_enabled, url, account_id, inbox_name, is_sign_enabled, sign_delimiter, is_reopen_enabled, is_pending_enabled, is_merge_enabled, is_import_contacts, is_import_messages, import_days, is_auto_create, organization, logo, ignored_jids, webhook_url}`. Token nunca (write-only; o campo `token` sai da leitura — antes era devolvido vazio).
7. **Erros**: `{"error":{"code","message"}}` + `X-Request-Id` preservados em todas as rotas JSON. `last_error` de conexão/envio: `{code, message, occurred_at}` ou `null`; legado migra com `code:"legacy_error"`, `occurred_at:null`.
8. **Preservados fora do envelope JSON**: `GET /media/{id}` (bytes com Content-Type próprio), `204` sem corpo, `GET /manager*` (HTML/SPA + 301), `POST /chatwoot/webhook/{id}` (200 com objeto de confirmação externa), `/swagger/*`.
9. **Replay idempotente legado**: corpos `response_body` armazenados são convertidos por rota ao DTO novo (mesmo `id`/UUID, mesmo `http_status`) marcado com `X-Idempotent-Replay`, sem reexecutar efeito. Corpo não conversível → `410 {"error":{"code":"idempotency_response_expired"}}` (regra fechada no design §5).
10. **Status codes**: preservados por operação conforme coluna atual (201 create, 202 aceitos assíncronos, 204 deletes/disconnect, 200 demais). Erros preservados: 400 corpo, 401 credencial, 403 escopo/quota, 404 alvo, 409 conflito de estado/ambíguo/em progresso, 413 corpo, 422 validação/fingerprint, 429 rate limit (login/webhook chatwoot), 501 upstream não suporta, 503 instância desconectada em envio.

| Operação | Respostas no Swagger atual | Destino / situação |
|---|---|---|
| `POST /auth/login` | 200: sem schema; 400/401/413/429/500: errorEnvelope | `data.me {id,email,role}` (hoje `identityResponse` plano). 429 do LoginRateLimiter preservado. Sem campos removidos. |
| `POST /auth/logout` | 200: sem schema | `data` com `{status:"ok"}` (comando). Cookie de sessão apagado; sem mudança. |
| `GET /auth/me` | 200: sem schema; 401/500 | `data.me {id,email,role}` (mesmos campos, chave `me`). |
| `POST /chatwoot/webhook/{id}` | 200: object; 400/404/409/429/500 | **Preservado**: confirmação externa aberta por design; corpo objeto simples (`{"ok":true}`/ack), 429 do limiter próprio. Fora do contrato de recursos. |
| `GET /healthz` | 200: sem schema | `data` com `{status:"ok"}` (público). |
| `GET /instances` | 200; 401/403/500 | `data.items[].instance` (DTO público §3). `items` vazio é `[]`. |
| `POST /instances` | 201; 400/401/403/409/413/422/500 | `data.instance` (DTO público) + `data.instance_api_key` (string, entregue uma vez). Erros: 403 `quota_exceeded`, 409 `instance_name_taken`/`conflict`(external_ref), 422 `invalid_instance_name`/`unprocessable_entity`. |
| `GET /instances/stats` | 200; 401/403/404/409/500 | `data.stats {total, by_status:{connected,disconnected,pairing,error}}` (chave `stats`; contagens inalteradas). Query `?instance=` aceita UUID ou nome (contrato de alias preservado). |
| `GET /instances/{id}` | 200; 401/403/404/409/500 | `data.instance` (DTO público §3). `{id}` aceita UUID ou nome exato (alias). 409 `instance_name_ambiguous` preservado. |
| `DELETE /instances/{id}` | 204; 401/403/404/409/500 | **204 sem corpo** preservado; alias por nome preservado. |
| `PATCH /instances/{id}` | 200; 400/401/403/404/409/413/422/500 | `data.instance`. Entrada: `name`, `external_ref` (aceitos na escrita autorizada embora ocultos na leitura), `webhook:{url,enabled,events}` — entrada também migra para objeto aninhado `webhook`; omitido preserva, `url:""` limpa, `events:[]` limpa (regra ponteiros atual). |
| `DELETE /instances/{id}/apikey` | 204; 401/403/404/409/500 | **204 sem corpo** preservado. |
| `POST /instances/{id}/apikey/rotate` | 200; 401/403/404/409/500 | `data` com `{id, instance_api_key}` (segundo e último ponto de entrega da chave em claro). |
| `GET /instances/{id}/blocklist` | 200; 401/403/404/409/500 | `data.items[]` strings JID (leitura derivada, sem entidade própria; `items` vazio `[]`). |
| `POST /instances/{id}/blocklist` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando). Idempotente (middleware). |
| `POST /instances/{id}/calls/reject` | 200; 400/401/403/404/409/413/422/500/501 | `data` com `{rejected:bool}`. 501 `not_supported` preservado. |
| `PUT /instances/{id}/chats/default-disappearing` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando). Idempotente. |
| `POST /instances/{id}/chats/mark-read` | 200; 400/401/403/404/409/413/422/500 | `data` com `{marked_read:bool}` (comando). |
| `GET /instances/{id}/chats/{chat}/disappearing` | 200; 401/403/404/409/422/500 | `data` com `{chat, duration_seconds, found}` (leitura derivada upstream). |
| `PUT /instances/{id}/chats/{chat}/disappearing` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando). Idempotente. |
| `GET /instances/{id}/chatwoot` | 200; 400/401/403/404/409/500 | `data.chatwoot_config` (DTO §6). Sem config → `is_enabled:false` + defaults, `webhook_url` derivado; token ausente da leitura. |
| `PUT /instances/{id}/chatwoot` | 200; 400/401/403/404/409/413/422/500 | `data.chatwoot_config` (DTO §6, token write-only na entrada). **BREAKING** nomes de flags: entrada sai de `enabled/sign_msg/name_inbox/...` para `is_enabled/is_sign_enabled/inbox_name/...`. |
| `POST /instances/{id}/chatwoot/command` | 200; 400/401/403/404/409/413/500 | `data` com `{ok:true}` (comando interno; corpo atual `{"ok":true}` preservado). |
| `POST /instances/{id}/chatwoot/import` | 202; 400/401/403/404/409/500 | `data` com `{imported:int}` (contagem só de mensagens; preservado). |
| `POST /instances/{id}/connect` | 200; 401/403/404/409/500 | `data.connection {status, qr_code?, qr_expires_at?}` — comando cujo resultado é o estado de conexão; `qr_*` só em pairing. |
| `GET /instances/{id}/contact-link` | 200; 401/403/404/409/500 | `data` com `{link}` (leitura derivada upstream). |
| `POST /instances/{id}/contacts/check` | 200; 400/401/403/404/409/413/422/500 | `data.items[] {phone, jid, is_on_whatsapp, last_seen?}` (leitura derivada upstream; `items` vazio `[]`). Sem idempotência (read single-signal). |
| `GET /instances/{id}/contacts/{jid}/business` | 200; 401/403/404/409/422/500 | `data` com `{jid, name, description, verified_name}` (leitura derivada). |
| `GET /instances/{id}/contacts/{jid}/devices` | 200; 401/403/404/409/422/500 | `data` com `{jid, devices[]}` (leitura derivada). |
| `GET /instances/{id}/contacts/{jid}/photo` | 200; 401/403/404/409/422/500 | `data` com `{jid, url, version}` (leitura derivada). |
| `POST /instances/{id}/contacts/{jid}/subscribe` | 200; 401/403/404/409/422/500 | `data` com `{subscribed:bool}` (comando). Sem idempotência (sinal de presença). |
| `POST /instances/{id}/disconnect` | 204; 401/403/404/409/500 | **204 sem corpo** preservado. |
| `GET /instances/{id}/groups` | 200; 401/403/404/409/500 | `data.items[].group` (DTO group §abaixo) + `data.next_cursor`. Group público: `{jid, name, description?, participants[] {jid,is_admin,is_super_admin}, participant_count, invite_code?, updated_at}`. |
| `POST /instances/{id}/groups` | 201; 400/401/403/404/409/413/422/500 | `data.group` (criação devolve o grupo com `jid` atribuído). |
| `GET /instances/{id}/groups/invite-preview` | 200; 401/403/404/409/422/500 | `data.group` parcial (`{jid,name,participant_count,...}` conforme preview upstream) — leitura derivada com a mesma forma de `group`. |
| `POST /instances/{id}/groups/join` | 200; 400/401/403/404/409/413/422/500 | `data` com `{jid}` (comando; devolve o JID do grupo entrado). |
| `GET /instances/{id}/groups/{group_id}` | 200; 401/403/404/409/500 | `data.group` (completo com `participants`, `invite_code` quando disponível). |
| `PATCH /instances/{id}/groups/{group_id}` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando; update parcial name/description). |
| `GET /instances/{id}/groups/{group_id}/invite` | 200; 401/403/404/409/500 | `data` com `{invite_code}` (leitura derivada). |
| `POST /instances/{id}/groups/{group_id}/invite/reset` | 200; 401/403/404/409/500 | `data` com `{invite_code}` (comando devolve o novo código). |
| `POST /instances/{id}/groups/{group_id}/leave` | 200; 401/403/404/409/500 | `data` com `{left:bool}` (comando). |
| `POST /instances/{id}/groups/{group_id}/participants` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando add/remove/promote/demote). |
| `PUT /instances/{id}/groups/{group_id}/photo` | 200; 401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando; multipart foto). |
| `GET /instances/{id}/groups/{group_id}/requests` | 200; 401/403/404/409/500 | `data` com `{participants[] {jid,is_admin,is_super_admin}}` (leitura derivada; pendentes de aprovação). |
| `POST /instances/{id}/groups/{group_id}/requests` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando approve/reject). Idempotente. |
| `PATCH /instances/{id}/groups/{group_id}/settings` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` (comando; announce/locked/join_approval/member_add_mode). Idempotente. |
| `GET /instances/{id}/messages` | 200; 400/401/403/404/409/500 | `data.items[].message` (DTO §4) + `data.next_cursor` (paginação por cursor preservada, `limit`/`cursor` query). |
| `POST /instances/{id}/messages` | 202; 400/401/403/404/409/413/422/500/503 | `data.message {id, send_status:"queued"}` (forma completa §4 disponível no aceite; substitui `{message_id,status}`). Idempotente; replay legado convertido com mesmo UUID (§9). 503 quando offline não enfileira. |
| `POST /instances/{id}/messages/contact` | 202; 400/401/403/404/409/413/422/500/503 | `data.message` (aceite, §4). Idempotente + replay legado §9. |
| `POST /instances/{id}/messages/edit` | 200; 400/401/403/404/409/413/422/500 | `data` com `{message_id}` (comando upstream; **não** é a entidade fila — é o wa_id da mensagem editada). Idempotente. |
| `POST /instances/{id}/messages/location` | 202; 400/401/403/404/409/413/422/500/503 | `data.message` (aceite, §4). Idempotente + replay legado. |
| `POST /instances/{id}/messages/media` | 202; 400/401/403/404/409/422/500/503 | `data.message` (aceite, §4, com `media_id` preenchido). Multipart; idempotente + replay legado. |
| `POST /instances/{id}/messages/revoke` | 200; 400/401/403/404/409/413/422/500 | `data` com `{revoked:bool, reason?}` (comando; usa `wa_id` real — nunca marcador `pending:`/chatwoot). |
| `POST /instances/{id}/messages/text` | 202; 400/401/403/404/409/413/422/500/503 | `data.message` (aceite, §4). Idempotente + replay legado. |
| `GET /instances/{id}/messages/{message_id}` | 200; 401/403/404/409/500 | `data.message` (DTO §4 completo: `last_error` estruturado, `wa_id`, `delivered_at`, `read_at`). `{message_id}` é o UUID interno da fila. |
| `GET /instances/{id}/newsletters` | 200; 401/403/404/409/500 | `data.items[].channel` + `data.next_cursor`. Channel público: `{channel, title, description?, follower_count, updated_at}`. |
| `POST /instances/{id}/newsletters` | 201; 400/401/403/404/409/413/422/500 | `data.channel` (criação devolve o canal com `channel` JID atribuído). Idempotente. |
| `POST /instances/{id}/newsletters/follow` | 200; 400/401/403/404/409/413/422/500 | `data` com `{followed:bool}` (comando). |
| `POST /instances/{id}/newsletters/unfollow` | 200; 400/401/403/404/409/413/422/500 | `data` com `{followed:false}` (comando; mesmo campo `followed` do follow). |
| `GET /instances/{id}/newsletters/{channel}` | 200; 401/403/404/409/500 | `data.channel` (mesma forma do item de coleção). |
| `GET /instances/{id}/newsletters/{channel}/messages` | 200; 401/403/404/409/422/500 | `data.items[] {server_id, content, timestamp}` + `data.next_cursor` (leitura derivada upstream; não usa entidade `message` — não é a fila). |
| `POST /instances/{id}/newsletters/{channel}/mute` | 200; 400/401/403/404/409/413/422/500 | `data` com `{muted:bool}` (comando). Idempotente. |
| `POST /instances/{id}/newsletters/{channel}/reactions` | 200; 400/401/403/404/409/413/422/500 | `data` com `{reacted:bool}` (comando). Idempotente. |
| `GET /instances/{id}/newsletters/{channel}/updates` | 200; 401/403/404/409/422/500 | `data.items[] {server_id, content, timestamp}` (leitura derivada; `items` vazio `[]`). |
| `POST /instances/{id}/newsletters/{channel}/viewed` | 200; 400/401/403/404/409/413/422/500 | `data` com `{viewed:bool}` (comando). Idempotente. |
| `POST /instances/{id}/numbers/check` | 200; 400/401/403/404/409/413/500/503 | `data` com `{exists, jid, normalized}` (leitura derivada; regra 9º dígito BR preservada). 503 offline. |
| `POST /instances/{id}/pair-phone` | 200; 400/401/403/404/409/413/422/500 | `data` com `{pairing_code, expires_at}` (comando de pareamento por telefone). |
| `POST /instances/{id}/presence` | 200; 400/401/403/404/409/413/422/500 | `data` com `{sent:bool}` (comando). |
| `GET /instances/{id}/privacy` | 200; 401/403/404/409/500 | `data` com `{last_seen, profile_photo, status, read_receipts, groups_add}` (leitura derivada upstream). |
| `PUT /instances/{id}/privacy` | 200; 400/401/403/404/409/413/422/500 | `data` com `{updated:bool}` — hoje o handler devolve a `privacyResponse` lida; **fechado**: devolve o objeto privacy completo `{last_seen,profile_photo,status,read_receipts,groups_add}` após aplicar (leitura pós-escrita, mesma forma do GET). |
| `GET /instances/{id}/profile` | 200; 401/403/404/409/500 | `data` com `{name, status_text, photo_url}` (leitura derivada upstream). |
| `PATCH /instances/{id}/profile` | 200; 400/401/403/404/409/413/422/500/501 | `data` com `{name, status_text, photo_url}` (mesma forma do GET após aplicar; ponteiros distinguem omitido de vazio). 501 preservado. |
| `PUT /instances/{id}/profile/photo` | 200; 400/401/403/404/409/413/422/500/501 | `data` com `{updated:bool}` (comando; multipart). 501 preservado. |
| `GET /instances/{id}/qr` | 200; 401/403/404/409/500 | `data.connection {status, qr_code?, qr_expires_at?}` (mesma forma do connect; só em pairing). |
| `GET /instances/{id}/status` | 200; 401/403/404/409/500 | `data.connection {status, last_error:{code,message,occurred_at}|null, last_connected_at|null}` — substitui `statusResponse` plano; `whatsapp_jid` **removido** da leitura pública (campo interno). |
| `GET /instances/{id}/status/privacy` | 200; 401/403/404/409/500 | `data` com `{mode, jids[]}` (leitura derivada; `jids` vazio `[]`). |
| `GET /instances/{id}/status/updates` | 200; 401/403/404/409/500 | `data.items[] {id, type, text?, caption?, created_at}` (leitura derivada dos próprios status; `items` vazio `[]`). |
| `POST /instances/{id}/status/updates` | 202; 400/401/403/404/409/413/422/500 | `data` com `{message_id, status}` (comando upstream; `message_id` é wa_id do status — não é a fila). Idempotente. |
| `POST /instances/{id}/status/updates/media` | 202; 400/401/403/404/409/422/500 | `data` com `{message_id, status}` (comando; multipart imagem/vídeo). Idempotente. |
| `DELETE /instances/{id}/status/updates/{status_id}` | 200; 401/403/404/409/500 | `data` com `{deleted:bool}` (comando; `{status_id}` é wa_id). |
| `GET /manager` | 301: string; 503: string | **Preservado**: redirect para `/manager/`; 503 quando o embed não foi buildado. |
| `GET /manager/` | 200: string; 503: string | **Preservado**: SPA HTML + fallback; fora do envelope JSON. |
| `GET /media/{id}` | 200: file; 401/403/404/500 | **Preservado**: bytes com `Content-Type` do `mime_type` + `X-Content-Type-Options: nosniff`; backend lê do MinIO (bucket/object_key) — 404 quando `object_deleted_at` preenchido ou objeto ausente. |
| `GET /readyz` | 200: sem schema; 503: sem schema | `data` com `{status:"ready"|"unready", checks:{nome:"ok"|"failed"}}` (público). |
| `GET /users` | 200; 401/403/500 | `data.items[].user` (DTO §5; `items` vazio `[]`). Admin-only. |
| `POST /users` | 201; 400/401/403/409/413/422/500 | `data.user` (DTO §5). Entrada `instance_quota` → renomeado `instance_limit` (**BREAKING** no request também); `instances_used` computado (=0 na criação). 409 `email_taken`. |
| `GET /users/{id}` | 200; 401/403/404/500 | `data.user` (DTO §5). |
| `DELETE /users/{id}` | 204; 401/403/404/409/500 | **204 sem corpo**; 409 quando o usuário ainda possui instâncias (regra preservada — FK SET NULL é só rede de segurança). |
| `PATCH /users/{id}` | 200; 400/401/403/404/413/422/500 | `data.user` (DTO §5). Entrada `{instance_limit}` (**BREAKING**, ex-`instance_quota`; `null`/`0` = ilimitado preservado). |

## Divergências handler↔Swagger registradas

- `POST /instances/{id}/chatwoot/command` existe no mux e no Swagger; é rota interna do conector, mantida.
- `GET /users` e `POST /users` existem (admin). `PATCH /users/{id}` aceita `instance_quota` hoje — a mudança para `instance_limit` é **BREAKING** intencional.
- As 8 rotas idempotentes de envio (`messages`, `messages/text|location|contact|media`, `status/updates`, `status/updates/media`) têm cache legado a converter; as demais rotas idempotentes (grupos, newsletters, chats, blocklist, edits) guardam corpos de comando cujo formato não muda — replay serve o corpo armazenado sem conversão.
- `POST /instances/{id}/messages/edit` e `revoke` operam por `wa_id`; nunca aceitam `wa_key` sintética `pending:*` (design §4).
- Nenhuma rota autenticada presente no mux ficou fora do Swagger; nenhuma rota do Swagger ficou sem handler.

## Critério de fechamento — verificado

- Todas as 89 operações têm destino explícito na tabela; nenhuma ficou com "Fechar DTO" pendente.
- Envelope `data`/`error` + `X-Request-Id` preservados; `GET /media/{id}`, `204`, `/manager*`, `/chatwoot/webhook/{id}` e `/swagger/*` fora do contrato de recursos, conforme regra 8.
- Parâmetros operacionais (alias por nome, `?instance=`, cursor, `limit`) preservados — nenhum removido por cosmética de documentação.
