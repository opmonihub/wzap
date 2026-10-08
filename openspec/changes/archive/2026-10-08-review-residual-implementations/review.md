# Revisão da codebase — evidências e escopo

Data: 2026-10-07. Base: f9d1300. Pedido: revisão com subagentes por direção e correção/refatoração de problemas residuais. Três revisores read-only na primeira onda; segunda onda do núcleo para concluir lacunas; implementers por área e revisão cruzada prevista. Códigos abaixo identificam causas independentes, não quantidade de asserts.

## Baseline

Go 1.26: `go test ./... -count=1`, vet, build e golangci-lint v2.13.2 passaram, 0 issues de lint. HTTP/auth focused do revisor passou. Manager: Vitest 7/7 e node:test de instanceName 7/7, lint/typecheck/build passaram; 2 warnings preexistentes. Integrações opcionais desabilitadas nessa baseline. Suites verdes não exercitavam os casos abaixo.

## API/autenticação — revisor review_api

| ID | Prioridade | Origem na base | Cenário e verificação planejada |
| --- | --- | --- | --- |
| A1 | P1 | internal/httpapi/core/middleware.go:134 | Admin removido/demovido mantém role JWT por 12h; login real, remoção/role atual e request privado demonstram 401/403 esperado. |
| A2 | P2 | internal/httpapi/core/idempotency.go:77 | PUT/PATCH envoltos no middleware ignoram chave; repetir timer/trava deve executar setter apenas uma vez, 422 divergente e 409 em progresso. |
| A3 | P2 | internal/httpapi/core/idempotency.go:334 | Pattern remove também group/channel, causando replay de outro recurso; dois alvos com mesma chave/body devem divergir sem segundo efeito. |
| A4 | P2 | internal/httpapi/core/idempotency.go:174 | Replay corrompido devolve sucesso; o teste existente confirmava truncated/empty/plaintext com 202; corrigir expectativa para 500 sem efeito/release. |
| A5 | P2 | internal/httpapi/core/instances.go:47 | Primeiro Decode aceita segundo JSON/junk ou padding além do limite; interação com fingerprint limitado pode colidir; exigir 400/413 e zero efeitos. |
| A6 | P2 | messages/messages.go:487; statuses/status.go:219; core/idempotency.go:394 | Query prevalece sobre multipart não fingerprintado e ordenar duplicatas ignora valor efetivo; testar query/ordem de fields/files. |
| A7 | P2 | internal/httpapi/authsession/login_ratelimit.go:52 | MaxLoginBuckets não limita mapa ativo; testar mais IPs que teto e budget/expiração sem evicção de entradas ativas. |

Cobertura: composição Chi, auth/RBAC, UUID/nome, idempotência, todos os handlers de recursos, DTOs/agregação, mídia, Chatwoot, readiness e Swagger. Fora dos achados: logout stateless documentado, corrida de cotas aceita no design e campos vazios previstos de contatos.

## Núcleo — review_core, primeira onda

| ID | Prioridade | Origem na base | Reprodução confirmada |
| --- | --- | --- | --- |
| C1 | P1 | webhook/deliver.go:67; webhook/ssrf.go:182; chatwoot/client/client.go:40 | Redirect 307 encaminha apikey/api_access_token a outro origin; dois httptest servers receberam credenciais sintéticas. |
| C2 | P2 | instance/service.go:327,377; storage/postgres/instances.go:213 | PATCH somente webhook ou identidade parcial regrava snapshot antigo, revertendo rename; enabled/events omitidos também são revertidos; probe com duas calls/barreiras. |
| C3 | P2 | session/whatsmeow/manager.go:200,224,227,320; chatwoot/inbound/webhook.go:603 | Restore JID sintético chega a logs/reason persistível e status Chatwoot ecoa DeviceJID; probes de parse/restore e confirmação status. |
| C4 | P2 | chatwoot/config/token.go:23; storage/postgres/chatwoot.go:100 | Plaintext começando enc:v1: passa sem cifrar e não suporta Seal/Open; probe de round trip falhou como esperado. |

Probes read-only via Go overlay `/tmp/wzap-core-review-probes/overlay.json`, sem arquivos da repo alterados ou segredos reais. Pacotes webhook/instance/whatsmeow tiveram REDs esperados; não foi integração externa.

## Núcleo — segunda onda

| ID | Prioridade | Origem na base | Reprodução confirmada |
| --- | --- | --- | --- |
| C5 | P1 | chatwoot/inbound/ssrf.go:188,161 | DNS de host diferente `[loopback, IP público confiável]` passa exceção e efetivamente alcança httptest loopback. |
| C6 | P2 | message/receipts.go:22,59; storage/postgres/messages.go:271 | Mesmo wa_id em duas instâncias recebe mesmo read_at; RED com Postgres real wzap_test:5435 e schema isolado postgrestest, limpo ao sair. |
| C7 | P2 | chatwoot/import/run.go:70,94; session/historysync.go:122,148 | Chunk recebido depois de snapshot desaparece no reset após import bem-sucedido; probe inseriu chunk no intervalo. |
| C8 | P2 | session/whatsmeow/historysync.go:31 | URI vazia ainda acumula contactos/chunks; probe com env ausente confirmou retenção. |
| C9 | P2 | message/outbox.go:319,328,349; storage/postgres/messages.go:285 | sent persistido seguido de Writer failure perde evento permanentemente; recovery não reemite, probe confirmou. |
| C10 | P2 | chatwoot/client/client.go:105; chatwoot/contacts/contacts.go:70 | contains inclui telefone alheio mais longo que vence o BR exato e recebe merge; probe confirmou seleção indevida. |
| C11 | P2 | chatwoot/inbound/webhook.go:609 | init só Get; instância fresca sem sessão apenas recebe confirmação de ausência; probe esperava criar/iniciar sessão. |

Overlay da segunda onda `/tmp/wzap-core-review-wave2/overlay.json`; sete REDs sintéticos e recibos com Postgres executado pelo coordenador. Os quatro candidatos iniciais foram confirmados, nenhum descartado.

Cobertura acumulada do núcleo: instance/session e paridade, app/inbound/runtime, rich senders/message/outbox/receipts, events/relay, media, webhook, SQL/repositories/migração inicial, locks, Chatwoot client/config/contacts/conversations/inbound/mapper/mirror/import. Sem WhatsApp real, API real Chatwoot ou schema de import externo.

Observações sem confirmação suficiente, que não autorizam correção especulativa nesta change: promoção concorrente de correlação pending, notas privadas em falhas assíncronas, extensão de desenho inbound e cursor newsletters em página final curta. Foram registradas pelo revisor como candidatos, não bugs demonstrados.

## Manager — review_manager

| ID | Prioridade | Origem na base | Reprodução confirmada |
| --- | --- | --- | --- |
| M1 | P2 | components/instances/ProfileCard.vue:45 | Script real envia nome inalterado ao salvar recado e recebe 501 antes do setter status_text. |
| M2 | P2 | pages/instances/[id].vue:67,151 | Detalhe por alias renomeado mantém route ID antigo; script real faz duas requests OldName e termina notFound apesar de NewName carregado. |
| M3 | P2 | components/instances/MessageComposer.vue:144 | Script real converte campos vazios para 0,0 e envia localização. |
| M4 | P2 | components/instances/ChannelsCard.vue:203 | Status de mídia sem legenda gera 1 registro, 0 linhas e nenhum empty/Delete. |
| M5 | P2 | components/instances/detail/InstanceSettingsSection.vue:245 | Revoke condicionado a cache/keySeen, e freshKey oculta ações; admin em outro navegador não revoga sem rotate. |

Cobertura: DTOs/composables/auth/middleware, CRUD/formulários, pairing/polling, mensagens/grupos/canais/status, profile/privacy, Chatwoot/settings/keys, config/proxy/serving. Reproduções isoladas com script real/Vue e mocks; sem navegador ou integração real. P3 de cobertura: pnpm test não inclui node:test instanceName; coordenador executa ambas as suites na validação, sem alterar tooling sem necessidade.

## Operações/empacotamento — coordenador

| ID | Prioridade | Origem | Reprodução confirmada |
| --- | --- | --- | --- |
| O1 | P2 | manager/manager.go:30 e ausência de manager/.output/public no índice | Go em worktree limpo falha no embed antes de compilar; comentário documenta placeholder force-added, mas git ls-files não possui arquivo; criar somente .gitkeep permite compilar sem Nuxt. |

Inicialização/shutdown/config/seed, Docker/Compose/Air, logger/version, gates e instruções operacionais foram lidos pelo coordenador. Divergências legadas de AGENTS.md local (media-migrate/cutover) pertencem às alterações do usuário e não são reescritas pela change.

## Entrega e limites

24 causas confirmadas (A1–A7, C1–C11, M1–M5, O1), todas com task de correção antes de edição. Implementers devem registrar RED/GREEN nos relatórios e revisores cruzados confirmar o resultado. Os paths/linhas acima referem-se à base e mudam com a correção.

Este arquivo descreve os achados; verify.md registrará o resultado final dos gates e quais integrações efetivamente rodaram. Não houve segundo gateway, reinício, deploy, alteração de workflows/migrações/dependências nem impressão de segredos reais.

## Segunda revisão dos patches — concluída

| Área | Reviewer independente | Resultado e evidência |
| --- | --- | --- |
| Task 1 API | review_manager | Aprovação após corrigir complemento A4: data:null/string/array era replay 202. Probe independente e 3 casos permanentes deram RED; validação mínima de objeto não-null passou nativos + probe original (core 0,011 s), com zero efeitos/releases e bytes íntegros. Demais regressões JWT/role, métodos/recursos/aliases, body limit, multipart e login capacity passaram. |
| Task 3 manager / O1 | review_manager | Nenhum P1/P2 confirmado no patch; 41 testes de 5 SFCs passaram. Harness avalia script/template reais, cache somente de compilação. Probe independente do pacote manager só com .gitkeep compilou e confirmou Built=false/503; helper de build e placeholder conferidos. |
| Task 2 core | review_manager | Nenhum P1/P2 no patch; regressões em 9 pacotes e positivos BR/merge/token passaram. DB isolado sem skips nos grupos selecionados: restore privacy/history option, snapshot import sucesso/conta inválida/trigger SQL e sealing/prefix/Get→Put/keyless. |
| Task 6 receipts/durability | fix_core + coordenador | Nenhum P1/P2 no patch. Leitura de transação, outcome matching, notifier e proteção do lote; seleção independente -race em message/postgres/webhook com DB passou (38,682/45,399/1,503 s). Probe extra -race count10 de cancelamento protege/libera todo lote, um Send e nenhum fan-out sem confirmação. |

O complemento A4 é refinamento da mesma causa de integridade de replay, sem aumentar as 24 causas. Também corrigida fixture preexistente de reconnect: sleeper instantâneo concluía retry antes da asserção; reproduziu 2 falhas sob -race count100 e passou 100 vezes após barreira por canais, sem alteração de produção/timeouts.

Todos os patches têm relatórios RED/GREEN em implementation/{api,core,manager,durability,operations}.md. A validação global e integração local constam de verify.md.
