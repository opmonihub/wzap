# Decisões aprovadas — 2026-10-06

## Origem e ordem da discussão

O pedido partiu da necessidade de uma leitura única de configuração da instância no console: hoje `webhook` é o único bloco de configuração na representação pública, e Chatwoot, perfil, privacidade, status privacy e timer de mensagens temporárias só existem em rotas próprias — com 409 nas rotas vivas para instâncias desconectadas. O contrato JSON foi cortado hoje pela change `remodel-storage-and-json-responses`, então qualquer reorganização é uma quebra consciente com consumidor único e conhecido (Manager Nuxt).

Fonte da aprovação: `/home/obsidian/.cursor/plans/instance-config-aggregation_886a900e.plan.md`, que traz o JSON alvo, as fontes de dados, a semântica de blocos nulos e a lista de fora de escopo. Este registro captura as decisões e os trade-offs discutidos; o desenho consolidado fica em `design.md`.

## Representação e o move do webhook

Foi aprovado acrescentar dois blocos à instância pública: `integration` (`webhook` + `chatwoot_config`) e `settings` (`default_disappearing`, `profile`, `privacy`, `status_privacy`). `webhook` sai da raiz de `instance` e passa a `integration.webhook` — **BREAKING**, marcado desde o pedido, porque o contrato foi cortado hoje e o único consumidor é o Manager, atualizado no mesmo corte.

Discutida e rejeitada a alternativa de manter `webhook` na raiz e só acrescentar `settings`: o objetivo é leitura única de configuração, e o custo da quebra é mínimo com consumidor único e migrável no mesmo release. Crição e atualização respondem a mesma representação (o plano inclui create/update na montagem), mantendo o PATCH de entrada restrito aos campos atuais — escopo é leitura agregada.

## Semântica de null e resiliência

Aprovado: `null` por bloco quando a fonte está indisponível. Uma instância desconectada nunca derruba a listagem — o item aparece com os blocos vivos `null` e os persistidos preenchidos. Rejeitada a alternativa de propagar 409/500 por falha de bloco (transformaria indisponibilidade parcial em falha total da listagem).

Blocos vivos (`profile`, `privacy`, `status_privacy`) só são buscados quando `connection.status == "connected"` — as rotas próprias respondem 409 fora desse estado (`internal/httpapi/profile.go`), então buscar em cascata só geraria erro e latência. Na listagem, a montagem usa concorrência limitada por requisição (cerca de 8): rejeitada a serialização (latência linear) e o paralelo ilimitado (rajada de sessões/queries).

## Fontes de dados

Nada novo além do eco do timer: `integration.webhook` já vem no agregado (`model.Instance.Webhook`); `integration.chatwoot_config` vem do `ChatwootConfigStore` (`internal/httpapi/server.go:38`); `profile`/`privacy` de `InstanceService.GetProfile`/`GetPrivacy` (`internal/instance/profile.go:15`, `:93`); `status_privacy` de `GetStatusPrivacy` (`internal/instance/parity_reads.go:206`).

## Chatwoot no agregado

Aprovado: `chatwoot_config` é `null` quando não há configuração persistida; quando há, o bloco repete a semântica de `GET /instances/{id}/chatwoot` — mesmos campos, sem `instance_id`, token nunca presente (reuso de `chatwootConfigResponse`, `internal/httpapi/chatwoot.go:86`). Observação registrada na discussão: o GET próprio de hoje sintetiza configuração vazia quando nada está persistido (`internal/httpapi/chatwoot.go:278-283`); o alinhamento pedido é de semântica de campos/segredos, e o `null` no agregado segue a regra de não inventar valor. Mudar o GET próprio ficou fora de escopo.

## default_disappearing

Hoje só existe o setter (`internal/instance/parity_writes.go:79`) e a rota responde `{updated: true}`. Discutidas três saídas para o `settings.default_disappearing`:

1. eco persistido do último PUT (escolhida);
2. leitura viva do aparelho (rejeitada: sem leitura confiável upstream e exigiria sessão conectada, quebrando a semântica de null);
3. valor default inventado (`0`) (rejeitada: esconderia "nunca configurado" de "desligado").

Aprovado: eco persistido em coluna nullable (migração `00009_default_disappearing.sql`), `null` quando nunca configurado. Assunção menor registrada durante o registro: o eco usa a mesma representação textual aceita pelo comando (`0`, `24h`, `168h`, `2160h`).

## Privacidade

Reafirmado sem mudança: `external_ref`, `owner_user_id`, JIDs e hashes continuam ocultos e persistidos; token Chatwoot continua write-only; a chave de instância continua só em criação/rotação. Os novos blocos herdam a mesma regra — os testes de contrato devem assegurar a ausência.

## Fora de escopo

Escrita de `integration`/`settings` via PATCH (as rotas próprias continuam sendo o caminho de escrita); paginação; eventos NATS/webhooks; mudanças de RBAC.

## O que este pedido autoriza agora

Somente a criação dos artefatos da change OpenSpec (`openspec/changes/instance-config-aggregation/`). Nenhum código, migração, documento fora da change ou implantação foi alterado; a implementação começa apenas com um novo pedido de apply.
