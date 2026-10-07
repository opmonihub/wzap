# Delta de respostas por rota — instance-config-aggregation

Delta sobre a matriz fechada hoje em `openspec/changes/archive/2026-10-06-remodel-storage-and-json-responses/response-matrix.md`. Só as rotas cujo corpo muda nesta change; as demais operações permanecem exatamente como na matriz do corte de 2026-10-06.

Regra fechada para a instância pública (design §1):

`instance` = `{id, name, connection:{status, last_error:{code,message,occurred_at}|null, last_connected_at|null}, integration:{webhook:{enabled, url|null, events[]}, chatwoot_config:<chatwootConfigResponse sem instance_id, sem token>|null}, settings:{default_disappearing:<eco "0"|"24h"|"168h"|"2160h">|null, profile:{name, status_text, photo_url}|null, privacy:{last_seen, profile_photo, status, read_receipts, groups_add}|null, status_privacy:{mode, ...}|null}, created_at, updated_at}`.

Ocultação preservada: `external_ref`, `owner_user_id`, `whatsapp_jid`, `device_jid`, hashes e token Chatwoot nunca aparecem. `null` por bloco quando a fonte está indisponível; blocos vivos (`profile`, `privacy`, `status_privacy`) só quando `connection.status == "connected"`.

| Operação | Respostas no contrato atual (corte 2026-10-06) | Destino / situação |
|---|---|---|
| `GET /instances` | `data.items[].instance` = `{id, name, connection, webhook, created_at, updated_at}` | **BREAKING**: `webhook` sai da raiz → `integration.webhook`; acrescenta `integration.chatwoot_config` (`null` sem config persistida) e `settings` com `default_disappearing` (eco do último PUT, `null` se nunca configurado) e `profile`/`privacy`/`status_privacy` (só quando `connected`; `null` nos demais estados e em falha de fonte). Cada item é montado de forma independente com concorrência limitada (~8): instância desconectada ou fonte indisponível nunca derruba a resposta nem os demais itens. Erros preservados (401/403/500). |
| `GET /instances/{id}` | `data.instance` = `{id, name, connection, webhook, created_at, updated_at}` | Mesma representação agregada da listagem em `data.instance`. `{id}` aceita UUID ou nome exato (alias) e o `409 instance_name_ambiguous` é preservado. Erros preservados (401/403/404/409/500). |
| `POST /instances` | `data.instance` + `data.instance_api_key` | Representação `instance` também agregada (`integration`/`settings`, com `default_disappearing` `null` e blocos vivos `null` — instância nasce `disconnected`); entrega única da key preservada. |
| `PATCH /instances/{id}` | `data.instance` | Representação `instance` também agregada; entrada continua restrita aos campos atuais (`name`, `external_ref`, `webhook:{url,enabled,events}` — sem escrita de `integration`/`settings`). |

## Preservados sem mudança

- Rotas próprias de escrita/leitura de configuração: `PUT`/`GET /instances/{id}/chatwoot`, `PUT /instances/{id}/privacy`, `PATCH /instances/{id}/profile`, `GET /instances/{id}/status/privacy`, `PUT /instances/{id}/chats/default-disappearing` (continua respondendo `data` com `{updated:bool}`; só passa a persistir o eco lido em `settings.default_disappearing`).
- Envelope `{"data": ...}` / `{"error": {"code", "message"}}` + `X-Request-Id`, eventos v1, mídia em bytes, `204` sem corpo, HTML/Swagger e webhook aberto do Chatwoot.
