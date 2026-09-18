# Design — whatsmeow-parity-routes

## Contexto

A API expõe hoje revogação, mark-read, presença, pair-phone, grupos básicos,
follow/unfollow/get/list de canais, status, perfil/recado e privacidade. A
versão pinada do whatsmeow já oferece edição, moderação de grupos, diretório,
blocklist, escrita de canal e ajustes de conversa, sem superfície REST. O
desenho abaixo mostra como expor tudo em 3 fases sem quebrar contratos.

## Decisões

### D1 — Edição síncrona com idempotência de envio, só texto próprio

- Racional: edição é pontual e o upstream responde na hora; reutilizar o
  middleware de idempotência existente evita fila/outbox nova e mantém a
  semântica que o consumidor já conhece (`202` do envio vira `200` aqui
  porque não há ciclo de entrega — decisão consciente, documentada no
  Swagger).
- Alternativa considerada: enfileirar edição no outbox como mensagem
  (`queued`→`sent`). Rejeitada: edição referencia mensagem já entregue e
  falhar silenciosamente via retry confundiria o consumidor; erro síncrono
  (`404`/`422`) é mais útil.

### D2 — Leituras primeiro (Fase 1), escritas depois (Fases 2–3)

- Racional: leituras (`joined groups`, `invite-preview`, diretório,
  blocklist, mensagens de canal) não têm efeito colateral e validam
  normalização de JID, paginação e mapeamento de erros antes das escritas.
- Alternativa considerada: uma fase única com tudo. Rejeitada: aumenta o
  blast radius (moderação de grupo e mute de canal afetam terceiros) e
  dificulta revisão.

### D3 — Reuso do `Session` como única porta para o protocolo

- Racional: todo acesso ao whatsmeow passa pelos métodos novos do contrato
  de sessão; o transporte nunca importa tipos da lib. Isso preserva a
  fronteira transporte → domínio → sessão e permite fakes nos testes.
- Alternativa considerada: handlers chamando o client direto. Rejeitada:
  vazaria tipos da lib para o HTTP e quebraria o isolamento testado.

### D4 — Erros mapeados sem vazar o upstream

- `ErrNotFound` → `404`, `ErrForbidden` → `403`, `ErrNotConnected` → `409`,
  conteúdo inválido → `422`, `ErrUnsupported` → `501 not_supported`. Nenhuma
  mensagem de erro do protocolo chega ao cliente; logs carregam só IDs
  opacos (`instance_id`, `message_id`), nunca JID/telefone/token.

### D5 — Paginação por cursor opaco, limites conservadores

- Lista de grupos e mensagens de canal usam cursor opaco (`next_cursor`) com
  `limit` padrão 50 / máximo 100; lote de contatos limitado a 50 e
  `server_ids` a 100. Evita varreduras caras no upstream e respostas
  gigantes.
- Alternativa considerada: paginação por offset. Rejeitada: o upstream é
  cursor-based; offset exigiria buffer local e stale reads.

### D6 — Sem eventos novos nesta change

- As operações são síncronas e observáveis pela resposta; nenhum subject
  NATS novo é criado (edição inbound já existe como evento). Webhook
  continua no default pinado. Novos eventos, se necessários (ex. pedidos de
  entrada), vão em change dedicada de inbound.

## Fronteiras preservadas

- Transporte (`internal/httpapi/`): rotas, envelopes, dual auth, RBAC,
  idempotência, Swagger. Nenhuma regra de negócio nova aqui.
- Domínio/serviços (`internal/message/`, `internal/instance/`): validação de
  conteúdo, resolução de JID (incl. regra do 9º dígito), ownership/quotas.
- Sessões (`internal/session/` + adaptador whatsmeow): única camada que fala
  o protocolo; traduz erros para `Err*` classificados.
- Eventos (`internal/events/`): intocados — sem subjects novos.
- Mídia (`internal/media/`): intocada — foto de perfil alheia é URL do
  upstream, não upload local.
- Armazenamento (`internal/storage/`): sem migração prevista; metadados de
  grupo/canal reusam o cache de refresh existente (write-through, nunca
  fonte).

## Riscos

- [Risk] Permissão de grupo ambígua no upstream (admin vs dono) → Mitigação:
  mapear qualquer recusa do protocolo para `403` genérico + teste com fake
  que simula `ErrForbidden`.
- [Risk] Prévia de convite usada para enumeração de grupos → Mitigação: sem
  listagem pública, só por código exato; rate limit existente do HTTP mantido;
  documentar uso responsável no Swagger.
- [Risk] Lote de contatos com custo alto no upstream → Mitigação: cap de 50,
  validação antes de tocar a sessão, `422` sem efeito parcial.
- [Risk] Reação/viewed em canal com `server_id` inválido → Mitigação:
  `404` por item desconhecido, sem aplicação parcial.
- [Risk] Temporizador com duração fora do suportado pelo protocolo →
  Mitigação: allowlist fechada (`0`/24h/7d/90d), `422` fora dela.
- [Risk] Divergência entre foto/privacidade alheia e cache local →
  Mitigação: leitura sempre live, sem cache novo; documentar best-effort.
