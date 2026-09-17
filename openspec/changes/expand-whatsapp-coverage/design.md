## Context

Ver `proposal.md` (Why) para a motivação. Estado atual que molda o desenho:

- `Session` já tem `DeleteMessage`, `MarkRead`, `SendPresence`, `PairPhone`
  implementados no adapter whatsmeow, sem rota REST correspondente.
- REST em `ServeMux` stdlib: rotas na raiz atrás de dual auth (sessão JWT OU
  header `apikey:` global/por instância), RBAC/ownership por dono imutável,
  envelopes `{"data"}` / `{"error"}` + `X-Request-Id`, idempotência de envio
  via `Idempotency-Key` (24 h por instância).
- Envio atual é assíncrono (`202` + outbox + relay NATS JetStream,
  at-least-once, `event_id` estável como `Nats-Msg-Id`); pareamento é
  síncrono (`connect`/`qr`/`status`/`disconnect`).
- Inbound vira evento versionado (`event_version: 1`) + mídia em
  `WZAP_DATA_DIR` + webhook por instância (envelope + `event` cru).
- Runtime suportado de 1 réplica; shutdown drena HTTP → outbox → cleaner →
  webhook worker → relay por último.

## Goals / Non-Goals

**Goals:**

- Fase 1 sem tocar na sessão: só transporte + validação + mapeamento de erro
  sobre métodos existentes.
- Fase 2 com paridade inbound/outbound por família: tudo que se envia e pode
  ser recebido tem evento correspondente documentado.
- Erros do upstream traduzidos em `409`/`422`/`501` estáveis, sem vazar
  detalhe interno nem quebrar o envelope.

**Non-Goals:**

- Nenhuma mudança em auth, ownership, cotas, idempotência de tipos atuais,
  envelope NATS ou ordem de shutdown.
- Nenhuma fila nova, tabela nova além de metadados de grupo/newsletter
  (aditiva) e nenhum requisito novo de infra.

## Decisions

### Fase 1 síncrona sobre a sessão, sem outbox

Revogar, marcar leitura, presença e código de pareamento são efeitos
imediatos e idempotentes por natureza; passam direto ao `Session` da
instância e respondem `200` (`204` onde não há corpo), com `409` quando
desconectada. Alternativas: enfileirar no outbox como mensagem (daria retry
e `202`, mas revogação/leitura/presença fora de janela perdem sentido e o
código de pareamento expira com o canal — recusado); worker dedicado
(complexidade sem ganho — recusado).

### `sender` obrigatório em grupo, opcional em 1:1

`MarkRead` exige autor em grupo (o protocolo roteia o recibo pelo autor);
em conversa direta o serviço completa com o próprio `chat` quando ausente.
Alternativa: exigir sempre (fricção no caso dominante — recusado); nunca
exigir (falha silenciosa em grupo — recusado).

### `pair-phone` exige `connect` antes

O código de 8 dígitos só existe dentro de um canal de pareamento aberto;
sem `connect` prévio a resposta é `409` (sem canal), não `422`. Expira junto
com o QR do canal. Alternativa: abrir o canal implicitamente (esconderia o
ciclo de vida e quebraria a simetria com o QR — recusado).

### Ricas assíncronas no outbox, resto síncrono

Enquete, reação, figurinha, lista e botões entram no aceite `202` +
idempotência existentes (são mensagens com `whatsapp_message_id` e recibos);
grupos, newsletters, status, perfil e privacidade são operações de
conta/conversa e respondem síncronas. Exceção registrada (ruling do
controller): publicar status responde `202` + `message_id` com replay
`X-Idempotent-Replay` via middleware (mesma semântica das mensagens), mas o
backing é síncrono fire-and-forget para o broadcast — sem retry de outbox,
sem rework de fila. Alternativa: tudo síncrono (perderia
retry/dedupe das mensagens — recusado); tudo assíncrono (polling de `GET
/messages/{id}` para trocar foto de perfil — recusado).

### Figurinha como `type=sticker` no `/messages/media`

Reuso do upload multipart existente com validação webp + limite
`MaxMediaBytes`; sem endpoint novo de upload. Alternativa: endpoint próprio
(duplicaria validação, idempotência e docs — recusado).

### Chamadas só observadas, nunca iniciadas

O companion não inicia chamada de voz/vídeo pelo protocolo suportado; o
escopo é evento inbound (oferta/aceite/recusa/fim) + rejeição da ativa
quando o upstream permitir, senão `501` documentado. Alternativa: prometer
discagem via REST (não entregável — recusado).

### Grupos/newsletters com metadados em cache aditivo

Metadados (assunto, foto, participantes, assinatura de newsletter) vivem em
tabelas novas aditivas com TTL/refresh sob demanda; fonte de verdade segue
o WhatsApp. Alternativa: sem persistência (cada `GET` bateria no protocolo
— latência e flake — recusado); espelhar tudo transacionalmente (consistência
impossível contra o upstream — recusado).

## Risks / Trade-offs

- [Risk] Upstream não suporta alguma operação (botões em certos clientes,
  rejeitar chamada, privacidade fina) → Mitigation: mapear para `501` com
  código estável + documentar suporte por operação no Swagger; sem vazar erro
  interno.
- [Risk] Revogação fora da janela falha silenciosamente no protocolo →
  Mitigation: traduzir resposta do upstream em `200` com campo `revoked:
  false` + motivo, nunca `500`; documentar a janela.
- [Risk] Presença abusada como heartbeat gera throttle/ban →
  Mitigation: validar `state` em allowlist, sem modo contínuo nesta change;
  documentar uso pontual.
- [Risk] `pair-phone` facilita enumeração de números →
  Mitigation: mesma fronteira dual auth + ownership das demais rotas; erro
  de número inválido genérico (`422`) sem distinguir existência.
- [Risk] Eventos novos (votos, reações, respostas) inundam NATS/webhook →
  Mitigation: reusar assinatura por tipo do webhook (`events`) com os tipos
  novos opt-in documentados; sem mudança no default.
- [Risk] Metadados de grupo divergem do upstream →
  Mitigation: cache com refresh sob demanda + `updated_at` exposto; nunca
  apresentar como fonte de verdade.
- [Trade-off] Operações síncronas não têm retry do outbox em troca de
  semântica imediata e erros fiéis do protocolo.

## Migration Plan

Só adições, sem migração de contrato:

1. Aplicar migrations aditivas (se houver, metadados de grupo/newsletter)
   via `wzap migrate` ou `WZAP_AUTO_MIGRATE=true`.
2. Regenerar Swagger (`swag init --parseInternal`) e conferir o check de
   frescura no CI.
3. Atualizar consumidores por família, atrás de feature flag própria;
   rollback = ignorar as rotas/tipos novos (código antigo os desconhece sem
   quebrar os atuais).

## Open Questions

- Mapeamento final `501` vs `422` para operação não suportada pelo upstream:
  proposta é `501` com código estável; confirmar no apply.
- `DELETE` de reação é emoji vazio ou endpoint próprio: proposta é emoji
  vazio no mesmo endpoint; confirmar contra o adapter.
- Janela de revogação e formato do `revoked: false`: confirmar contra o
  upstream no apply.
