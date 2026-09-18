## Why

A change `2026-09-17-expand-whatsapp-coverage` fechou a primeira onda de
paridade (revogar, mark-read, presença, pair-phone, grupos básicos,
newsletters follow/unfollow/get/list, status, perfil/recado, privacidade).
O `Session` (`internal/session/session.go`) e as rotas REST continuam sem
expor operações que a versão pinada do whatsmeow
(`v0.0.0-20260915211301-f376da267f95`, `go.mod`) já suporta de forma estável:

- Edição de mensagem de saída (`BuildEdit` + `SendMessage`): o inbound de
  edição já gera evento, mas não há rota para editar texto próprio.
- Moderação de grupos: pedidos de entrada (`GetGroupRequestParticipants` /
  `UpdateGroupRequestParticipants`), travas (`SetGroupAnnounce`,
  `SetGroupLocked`, `SetGroupJoinApprovalMode`, `SetGroupMemberAddMode`),
  lista de grupos da instância (`GetJoinedGroups`) e prévia de convite
  (`GetGroupInfoFromLink`).
- Diretório de contatos: `GetUserInfo`, `GetUserDevices`,
  `GetProfilePictureInfo` para JID arbitrário, `GetBusinessProfile` e
  `GetContactQRLink`.
- Bloqueios: `GetBlocklist` / `UpdateBlocklist` (bloquear/desbloquear).
- Canais (fase escrita): `CreateNewsletter`, `NewsletterToggleMute`,
  `NewsletterMarkViewed`, `NewsletterSendReaction`,
  `GetNewsletterMessages` / `GetNewsletterMessageUpdates`.
- Ajustes de conversa: `SetDisappearingTimer` /
  `SetDefaultDisappearingTimer`, `GetStatusPrivacy` e `SubscribePresence`.

Sem essas rotas, bots, suporte e comunidades precisam de acesso direto ao
protocolo fora do wzap, quebrando o modelo de instância única gerenciada
(RBAC/ownership, idempotência, envelopes, eventos versionados).

## What Changes

Fase 1 — leituras seguras + edição de mensagem (menor risco, sem efeito em
massa):

- Edição de saída: `POST /instances/{id}/messages/edit` (`chat`,
  `message_id`, `text`) → síncrono `200` com novo identificador quando o
  upstream aceita; idempotente via `Idempotency-Key` (mesma semântica do
  envio: replay devolve o original, divergência → `422`, concorrência →
  `409`). Só texto próprio editável; mídia/enquete não.
- Grupos (leitura): `GET /instances/{id}/groups` (lista os grupos da
  instância, paginação por cursor) e
  `GET /instances/{id}/groups/invite-preview?code=` (prévia sem entrar).
- Diretório (leitura): `POST /instances/{id}/contacts/check` em lote
  (`phones[]` → `jid`, `is_on_whatsapp`, `last_seen` best-effort),
  `GET /instances/{id}/contacts/{jid}/devices`,
  `GET /instances/{id}/contacts/{jid}/photo`,
  `GET /instances/{id}/contacts/{jid}/business`.
- Bloqueios (leitura): `GET /instances/{id}/blocklist`.
- Canais (leitura): `GET /instances/{id}/newsletters/{channel}/messages`
  (paginado por cursor do upstream) e
  `GET /instances/{id}/newsletters/{channel}/updates`.
- Conversas (leitura): `GET /instances/{id}/status/privacy` e
  `GET /instances/{id}/chats/{chat}/disappearing` (quando o upstream
  reportar; senão valor local vazio documentado).

Fase 2 — escritas de moderação (efeito visível, ainda escopo de grupo/chat):

- Grupos: `GET /instances/{id}/groups/{group_id}/requests` (lista pedidos),
  `POST .../requests` (`action: approve|decline`, `participants[]`),
  `PATCH .../settings` (`announce`, `locked`, `join_approval`,
  `member_add_mode`), tudo idempotente via `Idempotency-Key`.
- Bloqueios: `POST /instances/{id}/blocklist` (`action: block|unblock`,
  `jid`) idempotente.
- Conversas: `PUT /instances/{id}/chats/{chat}/disappearing`
  (`duration`, `0` desliga), `PUT /instances/{id}/chats/default-disappearing`
  e `POST /instances/{id}/contacts/{jid}/subscribe` (presença) — pontual,
  sem heartbeat (mesma regra de `wzap-presence`).
- Contato próprio: `GET /instances/{id}/contact-link` (`revoke=false|true`).

Fase 3 — escritas de canal (maior acoplamento ao produto canais):

- `POST /instances/{id}/newsletters` (criar canal),
  `POST .../{channel}/mute` (`muted: bool`),
  `POST .../{channel}/viewed` (`server_ids[]`),
  `POST .../{channel}/reactions` (`server_id`, `reaction`, vazio remove) —
  todas idempotentes via `Idempotency-Key`.

Transversal (todas as fases): dual auth + ownership existentes, envelope
`{"data": ...}` / `{"error": {"code", "message"}}` + `X-Request-Id`, `404`
instância inexistente ou recurso desconhecido no upstream (`ErrNotFound`),
`403` sem ownership ou sem permissão no grupo (`ErrForbidden`), `409`
instância não conectada, `422` conteúdo/estado inválido, `501`
`not_supported` só quando o upstream pinado não suportar (documentado por
operação no Swagger; sem vazar detalhe interno). Escritas usam o middleware
de idempotência existente; leituras não exigem `Idempotency-Key`. Eventos
novos seguem o envelope versionado (`event_version: 1`, `event_id` estável,
at-least-once, dedupe por `event_id`).

Sem **BREAKING**: nenhuma rota, envelope, evento, subject ou env existente
muda; tudo é aditivo. Se o apply revelar quebra inevitável, ela será
marcada com **BREAKING** antes do merge.

## Out-of-Scope

- O que esta change explicitamente NÃO faz:
  - Iniciar chamada de voz/vídeo pelo companion (só observação/rejeição já
    existentes).
  - `SetProfileName` / `SetProfilePhoto` (a lib pinada não expõe os setters;
    continuam `501` documentados).
  - Communities (`LinkGroup`/`UnlinkGroup`, `GetSubGroups`,
    `GetLinkedGroupsParticipants`) e broadcast lists: ficam para change
    dedicada de comunidades.
  - Sincronização de histórico/app-state (`FetchAppState`, `SendAppState`),
    pareamento passkey e `SendPeerMessage`: protocolo interno, sem REST.
  - Espelho Chatwoot dos novos tipos e importação de histórico cobrindo-os:
    changes dedicadas do conector.
  - Telas do Manager (`/manager`) para as novas rotas: change dedicada de
    frontend.
  - Rate limits novos, multi-réplica (locks seguem process-local, 1 réplica),
    webhooks globais, transferência de ownership, white-label.
  - Upgrade da versão pinada do whatsmeow (se algum método faltar, upgrade
    vai em change dedicada; nunca `latest` automático).

## Capabilities

### New Capabilities

- `wzap-message-edits`: editar texto próprio enviado.
- `wzap-group-moderation`: pedidos de entrada, travas do grupo, lista de
  grupos da instância e prévia de convite.
- `wzap-contacts-directory`: dados de contato (info em lote, dispositivos,
  foto alheia, perfil business, link próprio de contato).
- `wzap-blocklist`: consultar, bloquear e desbloquear contatos.
- `wzap-newsletter-ops`: criar, silenciar, marcar visualização, reagir e
  ler mensagens de canal.
- `wzap-chat-settings`: temporizador de desaparecimento (por chat + padrão),
  privacidade de status (leitura) e assinatura de presença.

### Modified Capabilities

- `wzap-outbound-messaging`: edição de saída entra no aceite idempotente
  (`Idempotency-Key`, replay, `422` divergente, `409` concorrência) sem mudar
  a semântica dos envios atuais.
- `wzap-groups`: escreve-through de metadados estendido às novas leituras
  live (lista/prévia) sem mudar o contrato de cache (log de refresh, nunca
  fonte).
- `wzap-newsletters`: novas escritas e leituras de mensagem no mesmo
  envelope/auth/RBAC, sem mudar follow/unfollow/get/list.
- `wzap-operations`: novas rotas na mesma fronteira de auth (dual auth,
  `401`/`403`), documentadas no Swagger; nenhuma pública nova.
