## Why

O `Session` (`internal/session/session.go`) já implementa `DeleteMessage`,
`MarkRead`, `SendPresence` e `PairPhone`, mas nenhuma rota REST os expõe: o
consumidor não consegue revogar mensagem, confirmar leitura, simular presença
nem parear por código de 8 dígitos sem QR. Na fase 2, famílias inteiras do
WhatsApp (enquetes, reações, figurinhas de saída, listas/botões, grupos,
newsletters, status/stories, chamadas, perfil e privacidade) não têm nem
método de sessão nem REST, o que trava casos de bot, suporte e comunidades.

## What Changes

Fase 1 — expor o que já existe na sessão (sem mudar semântica de pareamento,
envio ou eventos):

- Revogação: `POST /instances/{id}/messages/revoke` (`chat`, `message_id`) →
  revoga para todos, operação síncrona.
- Leitura: `POST /instances/{id}/chats/mark-read` (`chat`, `message_id`,
  `sender` opcional, obrigatório em grupo) → recibo de leitura.
- Presença: `POST /instances/{id}/presence` (`chat`, `state`: `composing`,
  `paused`, `available`, `unavailable`) → presença de chat ou de usuário.
- Pareamento por telefone: `POST /instances/{id}/pair-phone` (`phone`) →
  `{pairing_code, expires_at}`; exige canal de pareamento aberto (`connect`
  antes), expira junto com o QR.
- Todas sob dual auth + ownership existentes, envelope `{"data": ...}` /
  `{"error": {"code", "message"}}` + `X-Request-Id`, `404` instância
  inexistente, `403` sem ownership, `409` instância não conectada, `422`
  conteúdo inválido.

Fase 2 — novos métodos de sessão + REST + eventos/webhooks de entrada onde
houver inbound correspondente:

- Mensagens ricas: criar enquete, votar, reagir/remover reação, figurinha de
  saída (webp via `/messages/media` com `type=sticker`), lista e botões
  interativos (incl. PIX quando suportado).
- Grupos: criar, consultar, atualizar assunto/descrição/foto, gerenciar
  participantes e admins, entrar/sair por convite, códigos de convite.
- Newsletters (canais): seguir/deixar de seguir, consultar, listar.
- Status/stories e chamadas: publicar/consultar/apagar status (texto/imagem/
  vídeo), observar eventos de chamada (oferta/aceite/recusa/encerramento) e
  rejeitar chamada ativa quando suportado. Publicar status responde `202` +
  `message_id` com replay `X-Idempotent-Replay` via middleware (mesma
  semântica do Lote B), mas o backing é síncrono fire-and-forget para o
  broadcast — sem retry de outbox (ruling do controller: backing síncrono +
  202 mantidos, sem rework de fila).
- Perfil e privacidade: nome, foto, recado; última visualização, foto de
  perfil, status, confirmações de leitura e quem pode adicionar a grupos.
- Entrada (inbound): votos de enquete, reações recebidas, respostas de
  lista/botão, eventos de grupo, chamada e status passam a gerar eventos
  versionados (`event_version: 1`, `event_id` estável) via outbox + NATS +
  webhook, com entrega at-least-once e dedupe por `event_id` no consumidor.

Sem **BREAKING**: nenhuma rota, envelope, evento ou env existente muda; tudo
é aditivo. Se algum levantamento posterior exigir quebra, ela será marcada
com **BREAKING** antes do apply.

## Out-of-Scope

- O que esta change explicitamente NÃO faz:
  - Manager web (`/manager`): telas de QR por código, grupos, enquetes,
    status e privacidade ficam para change dedicada de frontend.
  - Espelho Chatwoot dos novos tipos inbound (votos, reações, listas,
    grupos, chamadas): change dedicada do conector.
  - Importação de histórico cobrindo os novos tipos.
  - Rate limits de msg/min e req/min; multi-réplica (locks seguem
    process-local, 1 réplica).
  - Iniciar chamada de voz/vídeo pelo companion (fora do suportado pelo
    protocolo; só observação/rejeição quando disponível).
  - Edição de mensagem de saídaChanging (só revogação; edição segue só inbound).
  - Webhooks globais; transferência de ownership; white-label.

## Capabilities

### New Capabilities

- `wzap-message-lifecycle`: revogar mensagem enviada e confirmar leitura.
- `wzap-presence`: presença de chat e de usuário.
- `wzap-phone-pairing`: pareamento por código de 8 dígitos.
- `wzap-rich-messaging`: enquetes, reações, figurinhas de saída, listas e
  botões interativos.
- `wzap-groups`: ciclo de vida e membros de grupos.
- `wzap-newsletters`: seguir, consultar e listar canais.
- `wzap-status-calls`: publicar/consultar/apagar status e observar/rejeitar
  chamadas.
- `wzap-profile-privacy`: perfil próprio e ajustes de privacidade.

### Modified Capabilities

- `wzap-outbound-messaging`: novos tipos aceitos no aceite/idempotência
  (`poll`, `reaction`, `sticker`, `list`, `buttons`) sem mudar a semântica
  dos tipos atuais.
- `wzap-inbound-events`: novos tipos de evento de entrada (votos, reações,
  respostas interativas, grupo, chamada, status) no mesmo envelope
  versionado.
- `wzap-instances`: `pair-phone` como alternativa ao QR no pareamento, sem
  mudar `connect`/`qr`/`status`/`disconnect`.
- `wzap-operations`: novas rotas na mesma fronteira de auth (dual auth,
  `401`/`403`), documentadas no Swagger; nenhuma pública nova.

## Impact

- Aditivo no REST (rotas e corpos novos) e nos eventos (tipos novos no mesmo
  envelope); NATS, outbox, idempotência e RBAC/ownership inalterados.
- Nova matriz de erros síncronos: `409` desconectado, `422` conteúdo/estado
  inválido, `501`/`422` quando o upstream não suporta a operação (a decidir
  no design, sem vazar detalhe interno).
- Persistência: grupos/newsletters podem exigir tabelas ou cache de
  metadados (migration goose aditiva); mensagens ricas reusam o outbox
  existente quando assíncronas.
- Dependência `whatsmeow` pode precisar de upgrade pinado em change dedicada
  se algum método não existir na versão atual; nunca `latest` automático.
- Docs: README, Swagger (`swag init --parseInternal` + check de frescura) e
  specs atualizadas; eventos novos documentados com exemplo de envelope.
