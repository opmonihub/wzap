## Context

Ver `proposal.md` para a motivação e `brainstorm.md` para o registro das aprovações. O contrato REST atual da instância foi cortado hoje (2026-10-06, change `remodel-storage-and-json-responses`): `instanceResponse` responde `{id, name, connection, webhook, created_at, updated_at}` (`internal/httpapi/dto.go:110`), com `connection` e `webhook` aninhados e campos internos ocultos. As configurações operacionais (Chatwoot, perfil, privacidade, status privacy, timer de mensagens temporárias) só existem em rotas próprias e não acompanham a leitura da instância. As rotas vivas de perfil/privacidade respondem 409 quando a instância está desconectada (documentação dos handlers em `internal/httpapi/profile.go`), o que impede uma agregação ingênua na listagem.

Fontes de dados já existentes (nenhuma nova leitura de domínio além do eco do timer):

| Bloco | Fonte | Custo |
|---|---|---|
| `integration.webhook` | `model.Instance.Webhook` (satélite já carregado nas leituras de instância) | zero — já está no agregado |
| `integration.chatwoot_config` | `ChatwootConfigStore` (`internal/httpapi/server.go:38`) | query simples por instância |
| `settings.profile` / `settings.privacy` | `InstanceService.GetProfile` (`internal/instance/profile.go:15`) / `GetPrivacy` (`internal/instance/profile.go:93`) | leitura viva via sessão |
| `settings.status_privacy` | `InstanceService.GetStatusPrivacy` (`internal/instance/parity_reads.go:206`) | leitura viva via sessão |
| `settings.default_disappearing` | hoje só existe o setter (`internal/instance/parity_writes.go:79`); passa a ter eco persistido | nova coluna nullable |

## Goals / Non-Goals

**Goals:** agregar `integration` e `settings` na representação pública da instância; manter a resiliência por bloco na listagem; persistir o eco de `default_disappearing` sem inventar valores; preservar privacidade e o contrato de eventos; atualizar Go, Swagger, Manager e README no mesmo corte.

**Non-Goals:** ver `proposal.md` (Out-of-Scope). Adicionalmente, este design não altera o contrato de escrita do `PATCH /instances/{id}` (que continua aceitando só os campos atuais) nem o comportamento próprio de `GET /instances/{id}/chatwoot`.

## Decisions

### 1. JSON público alvo (**BREAKING** no move do webhook)

Reproduzido exatamente como aprovado no plano (`instance-config-aggregation_886a900e.plan.md`):

```json
{"data":{"items":[{"instance":{
  "id": "...", "name": "G A CONT",
  "connection": {"status": "error", "last_error": {...}, "last_connected_at": "..."},
  "integration": {
    "webhook": {"enabled": false, "url": null, "events": ["message", "..."]},
    "chatwoot_config": { ...chatwootConfigResponse sem instance_id, token nunca presente... } | null
  },
  "settings": {
    "default_disappearing": null,
    "profile": {"name": "...", "status_text": "...", "photo_url": "..."} | null,
    "privacy": {"last_seen": "...", "profile_photo": "...", "status": "...", "read_receipts": "...", "groups_add": "..."} | null,
    "status_privacy": {"mode": "...", ...} | null
  },
  "created_at": "...", "updated_at": "..."
}}]}}
```

- `webhook` sai da raiz de `instance` e passa a `integration.webhook` (**BREAKING**; contrato cortado hoje, consumidor único = Manager).
- Individuais: `data.instance`; coleções: `data.items[].instance`. Criação (201) e atualização respondem a mesma representação (o plano inclui create/update na montagem dos blocos; o `PATCH` mantém a entrada restrita aos campos atuais — escopo: leitura agregada).
- Privacidade preservada: `external_ref`, `owner_user_id`, JIDs e hashes continuam ocultos; token Chatwoot continua write-only (reuso de `chatwootConfigResponse`, `internal/httpapi/chatwoot.go:86`, sem `instance_id`).
- Alternativa descartada: manter `webhook` na raiz e apenas acrescentar `settings` — rejeitada porque o objetivo é uma leitura única de configuração e o contrato foi cortado hoje, com um consumidor conhecido e atualizável no mesmo corte.

### 2. `null` por bloco quando indisponível

Cada bloco é composto de forma independente: indisponibilidade (sessão desconectada, falha de leitura viva, ausência de configuração persistida) produz `null` **apenas no bloco afetado**. Uma instância desconectada nunca derruba a listagem — o item continua presente com os blocos vivos `null` e os persistidos preenchidos. Erros de agregação são degradados, nunca propagados como status HTTP de falha da leitura da instância.

- Alternativa descartada: responder 409/500 quando um bloco falha — transformaria uma indisponibilidade parcial em falha total da listagem.

### 3. Blocos vivos só quando `connection.status == "connected"`

`profile`, `privacy` e `status_privacy` dependem de sessão ativa (rotas próprias respondem 409 fora de `connected`). A agregação só aciona essas leituras quando `connection.status == "connected"`; nos estados `disconnected`, `pairing` e `error` os três blocos são `null` sem chamada de sessão, evitando 409 em cascata e trabalho inútil.

### 4. Concorrência limitada na listagem (cerca de 8)

A montagem dos blocos em `GET /instances` usa concorrência limitada por requisição (grupo com limite ~8; implementação sugerida: `errgroup` com `SetLimit`), preservando a ordem dos itens. O limite é por requisição, não global: o runtime é de uma réplica e a listagem é autorizada/escopada, então o risco de rajada fica contido pelo próprio número de instâncias visíveis.

- Alternativa descartada: serializar as buscas vivas (latência linear no tamanho da lista) e buscas ilimitadas em paralelo (rajada de sessões/queries).

### 5. `chatwoot_config` é `null` quando não há configuração persistida

O bloco repete a semântica de `GET /instances/{id}/chatwoot` (mesmos campos de `chatwootConfigResponse`, sem `instance_id`, token nunca presente); quando não há configuração persistida, o agregado responde `null` em vez de fabricar um objeto. Observação factual: o `GET /instances/{id}/chatwoot` de hoje sintetiza configuração vazia (`is_enabled: false` + defaults) quando nada está persistido (`internal/httpapi/chatwoot.go:278-283`) — o alinhamento pedido é de semântica de campos/segredos, e a escolha `null` no agregado segue a mesma regra de "não inventar valor" do `default_disappearing`. Mudar o comportamento do GET próprio está fora de escopo.

### 6. `default_disappearing` como eco persistido do último PUT

Decisão: eco persistido do último valor aceito por `PUT /instances/{id}/chats/default-disappearing` em nova coluna nullable (`internal/storage/migrations/00009_default_disappearing.sql`), `null` quando nunca configurado — não inventar valor. Hoje só existe o setter (`internal/instance/parity_writes.go:79`) e a rota responde apenas `{updated: true}`.

- A escrita do eco acontece no comando existente do PUT (mesma operação que aplica o timer na sessão), no caminho de sucesso; o valor aceito pelo comando é o allowlist `0`, `24h`, `168h`, `2160h` (corpo `{"duration": "..."}`, `internal/httpapi/parity_writes.go:89`).
- Formato do eco (assunção menor registrada): a coluna e o JSON ecoam a mesma representação textual aceita pelo comando (`"0"`, `"24h"`, `"168h"`, `"2160h"`), com `null` para nunca configurado.
- Alternativas descartadas: derivar o valor do aparelho em leitura viva (o upstream não oferece leitura confiável do timer padrão e exigiria sessão conectada, violando a decisão 2); valor default inventado como `0` (esconderia "nunca configurado" de "desligado").

### 7. Fronteiras preservadas

A agregação acontece na camada de transporte (`internal/httpapi/`), orquestrando as fontes já existentes: repositório/`ChatwootConfigStore` para persistidos e `InstanceService` para as leituras vivas — sem novos estados de domínio, sem tocar eventos (`event_version: 1` intacto), mídia ou fila. A persistência do eco acrescenta um campo no contrato de storage (`internal/storage/repository.go`) sem alterar identidade, ownership ou unicidade existentes.

## Risks / Trade-offs

- [Consumidor do contrato cortado hoje quebra com o move do webhook] → consumidor único (Manager) atualizado no mesmo corte; **BREAKING** marcado no proposal, nos deltas e na matriz de respostas; testes de contrato fixam as chaves exatas.
- [Listagem lenta com muitas instâncias conectadas] → blocos vivos só quando `connected` e concorrência limitada (~8) por requisição; falha de fonte vira `null` no bloco, sem retry em cascata.
- [Token Chatwoot ou campos internos vazarem na agregação] → reuso do DTO write-only (`chatwootConfigResponse` sem `instance_id`) e testes de contrato com asserção de ausência de `token`, `external_ref`, `owner_user_id`, JIDs e hashes.
- [Eco de `default_disappearing` divergir do valor real no aparelho] → o campo é explicitamente o eco do último PUT (`null` = nunca configurado), documentado como tal; nada é inferido do estado do dispositivo.
- [Agregado divergir de `GET /instances/{id}/chatwoot`, que sintetiza defaults] → decisão 5 registra a escolha `null` no agregado e mantém o GET próprio intacto (fora de escopo); consumidores tratam `null` como integração não configurada.
- [Bloco vivo pendente de sessão whatsmeow atrasar a resposta unitária] → as três leituras vivas de uma instância podem rodar em paralelo entre si; qualquer erro vira `null` no bloco, sem propagar status de falha.

## Migration Plan

1. Aplicar `00009_default_disappearing.sql` (coluna nullable, expansão aditiva; rollback = remover a coluna) via `wzap migrate` ou `WZAP_AUTO_MIGRATE=true`, sem backfill — linhas existentes significam "nunca configurado" (`null`).
2. Deploy único do corte Go + Swagger + Manager: a representação muda de forma atômica para o consumidor; não há janela de compatibilidade dual-shape (consumidor único, migrado no mesmo release).
3. Verificar ao vivo `GET /instances` e `GET /instances/{id}` (instância desconectada e conectada) e o eco após um `PUT .../chats/default-disappearing`.
4. Sync dos deltas em `openspec/specs/` e arquivamento por último, conforme o workflow.
