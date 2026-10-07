# wzap

Gateway independente para WhatsApp. O wzap mantém sessões multi-instância,
expõe um contrato REST de comandos para aplicações consumidoras e publica os
eventos recebidos (mensagens, recibos, conexão, status de envio, votos de
enquete, reações, respostas interativas, eventos de grupo e chamadas) em um
stream durável do NATS JetStream.

## Visão e escopo

- Serviço Go independente (`module wzap`), operado por contas humanas
  (`admin`, `user`) e consumido por integrações via API keys. O serviço
  conhece apenas contas de operação e donos de instância: a instância é
  identificada por `id`, por um `external_ref` opcional e único e por um
  `owner_user_id` imutável cujo significado de negócio pertence ao
  consumidor.
- Múltiplas instâncias de WhatsApp em um processo, com sessões persistidas no
  Postgres (`whatsmeow`/sqlstore) e restauradas no boot.
- Envio assíncrono (`202` + `message_id`) de texto, localização, contato e
  mídia (imagem, vídeo, áudio/PTT e documento), além de mensagens ricas
  (enquete, reação, figurinha via `type=sticker` no upload, lista e botões),
  com idempotência por `Idempotency-Key` e estados observáveis.
- Ciclo de vida de mensagens (revogação, confirmação de leitura), presença,
  pareamento por código de telefone, grupos, newsletters, status/stories,
  observação/rejeição de chamadas, perfil e privacidade por instância
  conectada (operações síncronas, sem outbox; exceção: publicar status
  responde `202` + `message_id` com backing síncrono fire-and-forget, sem
  retry de outbox).
- Eventos publicados por instância com envelope versionado e entrega
  at-least-once via outbox transacional (sem perda em queda do broker ou do
  processo; duplicatas possíveis, deduplicáveis por `event_id`).
- Mídia recebida baixada automaticamente até o limite configurado, armazenada
  com checksum e servida por download autenticado enquanto não expira.

Fora de escopo na v1: webhooks globais, SQLite,
Redis, métricas Prometheus, escala horizontal, iniciar chamada de voz/vídeo
pelo companion (só observação/rejeição) e implementações de
consumidores. O serviço roda como **uma réplica**; locks são em memória e o
boot adquire um advisory lock do Postgres (segunda réplica aborta).

> **BREAKING (produto):** o contrato abaixo substitui o contrato headless
> anterior — header `apikey:` no lugar de `Authorization: Bearer`, rotas na
> raiz sem `/api/v1`, `WZAP_API_KEY` no lugar de `WZAP_SERVICE_TOKEN` e
> `media.url` sem prefixo. Consumidores da API antiga precisam migrar
> (detalhes em cada seção).

## Requisitos

- Go 1.26+ (o `go.mod` pina `go 1.26.0`) para build/testes locais.
- Postgres 18 (banco `wzap`; outro banco `_test` para os testes de
  integração) e NATS 2 com JetStream habilitado (`-js`).
- Docker + Docker Compose para a operação local completa (imagem distroless,
  binário estático, usuário não-root, healthcheck embutido).
- `golangci-lint` v2.13.2 (versão usada no CI) para o lint local.

## Configuração (`WZAP_*`)

Todas as variáveis são lidas do ambiente no boot. Vazio é tratado como não
informado. Variável obrigatória ausente ou valor malformado falha a
inicialização com mensagem nomeando a variável.

| Variável | Obrigatória | Padrão | Descrição |
| --- | --- | --- | --- |
| `WZAP_API_KEY` | sim | — | **BREAKING:** substitui `WZAP_SERVICE_TOKEN` (ignorada se presente). Key global de máquina, enviada no header `apikey:`, com acesso total. |
| `WZAP_JWT_SECRET` | sim | — | Segredo HMAC das sessões do manager (cookies JWT). **BREAKING:** mínimo de 32 caracteres (256 bits para HS256); segredos curtos falham o boot. `POST /auth/login` é limitado a 10 tentativas/min por IP (`429 rate_limited`). |
| `WZAP_ADMIN_EMAIL` | não | vazio | Seed do primeiro admin no boot quando não há contas; sem ela (ou sem `WZAP_ADMIN_PASSWORD`) nada é criado. |
| `WZAP_ADMIN_PASSWORD` | não | vazio | Senha inicial do admin seed (troque após instalar). |
| `WZAP_MAX_INSTANCES` | não | `0` | Teto global de instâncias; `0` = ilimitado. Acima responde `403 quota_exceeded`. |
| `WZAP_DEFAULT_USER_INSTANCE_QUOTA` | não | `0` | Cota padrão de instâncias por usuário; `0` = ilimitado; admin pode ajustar por conta. |
| `WZAP_DATABASE_URL` | sim | — | URL do Postgres, ex. `postgres://wzap:secret@postgres:5432/wzap?sslmode=disable`. |
| `WZAP_NATS_URL` | sim | — | URL do NATS, ex. `nats://nats:4222`. |
| `WZAP_HTTP_ADDR` | não | `:8080` | Endereço de escuta do HTTP. O subcomando `healthcheck` resolve host vazio/`0.0.0.0`/`::` para `127.0.0.1`. |
| `WZAP_PUBLIC_URL` | não | vazio | Base das URLs de download de mídia embutidas nos eventos (`media.url`). Sem ela a URL sai relativa (`/media/<id>`); na operação local use `http://127.0.0.1:8081`. |
| `WZAP_NATS_STREAM` | não | `WZAP` | Nome do stream JetStream. O stream cobre `wzap.>`. |
| `WZAP_EVENT_RETENTION_DAYS` | não | `7` | Retenção dos eventos no stream, em dias. |
| `WZAP_DATA_DIR` | não | `/data` | Raiz do cache local de mídia e do armazenamento legado em filesystem (modo sem `WZAP_S3_ENDPOINT`). |
| `WZAP_S3_ENDPOINT` | não | vazio | Endpoint S3/MinIO (ex. `http://minio:9000`). Com ele configurado os bytes de mídia vivem no object store e o data dir vira cache descartável; sem ele o backend de filesystem permanece ativo (janela de migração). Exige `WZAP_S3_ACCESS_KEY` e `WZAP_S3_SECRET_KEY`. |
| `WZAP_S3_BUCKET` | não | `wzap-media` | Bucket dos objetos de mídia, criado no boot quando ausente. |
| `WZAP_S3_REGION` | não | `us-east-1` | Região declarada ao cliente S3. |
| `WZAP_S3_ACCESS_KEY` | condicional | — | Access key do object store; obrigatória quando `WZAP_S3_ENDPOINT` está definido. |
| `WZAP_S3_SECRET_KEY` | condicional | — | Secret key do object store; obrigatória quando `WZAP_S3_ENDPOINT` está definido. |
| `WZAP_S3_USE_TLS` | não | `false` | Usa HTTPS quando o endpoint não traz scheme próprio. |
| `WZAP_MEDIA_TTL_SECONDS` | não | `7200` | TTL da mídia armazenada, em segundos (2 h). |
| `WZAP_MAX_MEDIA_BYTES` | não | `16777216` | Tamanho máximo de mídia recebida/enviada, em bytes (16 MiB). |
| `WZAP_OUTBOX_WORKERS` | não | `4` | Goroutines de envio do outbox. |
| `WZAP_HUMANIZE` | não | `false` | Simula presença/atraso antes do envio (humanização). |
| `WZAP_LOG_LEVEL` | não | `info` | Nível (`debug`, `info`, `warn`, `error`). |
| `WZAP_LOG_FORMAT` | não | `json` | Formato (`json` ou `console`; `text` é alias legado de `console`). |
| `WZAP_AUTO_MIGRATE` | não | `true` | Aplica as migrações embutidas no boot antes de aceitar tráfego. |

## Contrato REST

Base: `/` (raiz, sem prefixo). **BREAKING:** o prefixo `/api/v1` foi
eliminado; qualquer rota sob `/api/v1/*` responde `404`.

Toda rota da API exige credencial válida em um dos dois planos (mesmas rotas,
mesmos efeitos no escopo):

- Máquinas: header `apikey:` com a key global (acesso total) ou a instance
  key (controle total da própria instância, menos gerenciar a própria key).
  **BREAKING:** `Authorization: Bearer` não é mais aceito.
- Humanos (manager): sessão JWT em cookie httpOnly (`POST /auth/login` com
  email/senha, `POST /auth/logout`, `GET /auth/me`).

Ausente ou inválida responde `401`; válida sem escopo responde `403`.
Sucesso responde no envelope `{"data": ...}` e erro em
`{"error": {"code", "message"}}`; toda resposta carrega `X-Request-Id` (eco
do enviado ou gerado) e os logs da requisição usam o mesmo identificador.

### Saúde e prontidão (sem autenticação)

| Método e rota | Resposta |
| --- | --- |
| `GET /healthz` | `200` com `{"data":{"status":"ok"}}` enquanto o processo vive. |
| `GET /readyz` | `200` quando Postgres, migrações e NATS estão prontos; `503` com `{"data":{"status":"unready","checks":{...}}}` caso contrário. |

Públicas sem credencial: `GET /healthz`, `GET /readyz`, o console em
`/manager` e a documentação em `/swagger/*` (as chamadas de dados seguem as
regras de autenticação acima).

Subcomandos do binário: `wzap serve` (padrão), `wzap migrate` (aplica as
migrações e sai), `wzap media-migrate` (transfere mídia local para o object
store quando `WZAP_S3_ENDPOINT` está configurado, verificando SHA-256;
arquivos ilegíveis e rows sem cópia — sem arquivo e sem objeto no bucket —
falham o comando por linha) e `wzap healthcheck` (chama `/readyz` em loopback
e sai `0`/`1`; é o healthcheck do container).

O corte do remodel de storage tem um gate na migração `00010`: divergências
de identidade (`whatsapp_jid` vs `device_jid`) registradas em
`remodel_report` (`device_jid_conflicts`) bloqueiam `wzap migrate` e o boot
com `WZAP_AUTO_MIGRATE=true` até resolução explícita — o erro lista os casos;
o operador reconcilia `instance_connections.device_jid`, marca a row como
resolvida (`UPDATE remodel_report SET resolved_at = now() WHERE category =
'device_jid_conflicts' AND ref_id = ...`) e roda `wzap migrate` novamente. A
row fica como histórico de auditoria.

### Contas (só key global ou sessão `admin`; sem registro público)

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /users` | `{"email","password","role"}` → `201`; email duplicado → `409`. |
| `GET /users` | `200` com a lista. |
| `GET /users/{id}` | `200` com a conta; `404` se não existir. |
| `PATCH /users/{id}` | `{"instance_quota"}` → `200` (só a cota é editável). |
| `DELETE /users/{id}` | `204`; dono com instâncias → `409` (sem transferência, sem cascata). |

No primeiro boot sem contas e com `WZAP_ADMIN_EMAIL`/`WZAP_ADMIN_PASSWORD`,
o serviço cria o admin inicial (e adota as instâncias legadas sem dono);
sem as vars, nenhuma conta é criada.

### Instâncias

Todos os caminhos de instância aceitam `{id}` como UUID ou nome exato,
incluindo `POST /chatwoot/webhook/{id}`. Nomes diferenciam maiúsculas de
minúsculas; UUIDs têm precedência e continuam sendo a identidade de sessões,
keys, mensagens, eventos e idempotência. Nome inexistente ou referência
inválida → `404`; nome legado válido duplicado → `409 instance_name_ambiguous`.
Uma renomeação muda o alias imediatamente e preserva o acesso pelo UUID.

**BREAKING:** novos nomes e renomeações devem ser globalmente únicos e ter
1–64 caracteres ASCII: letras, dígitos, hífen ou underscore, começando e
terminando com letra ou dígito. O nome exato `stats` e qualquer valor aceito
como UUID (inclusive compacto) são reservados. Nome inválido →
`422 invalid_instance_name`; nome ocupado → `409 instance_name_taken`.
Nomes legados não são reescritos: seguem acessíveis pelo UUID e podem ficar
exatamente iguais em atualizações de outros campos, ou ser renomeados para
um nome válido disponível.

Exemplos para uma instância chamada `Loja_SP-1`:

```sh
curl -H 'apikey: dev-wzap-token' http://127.0.0.1:8081/instances/Loja_SP-1
curl -H 'apikey: dev-wzap-token' http://127.0.0.1:8081/instances/Loja_SP-1/status
curl -H 'apikey: dev-wzap-token' 'http://127.0.0.1:8081/instances/stats?instance=Loja_SP-1'
```

`GET /instances/stats?instance=<uuid-ou-nome>` conta somente o alvo autorizado,
com a mesma resposta `total` e `by_status` (quatro estados, total 1).
Sem query, conta a coleção autorizada. Instance keys continuam recebendo
`403` em stats; usuários não administradores recebem `403` para alvo de
outro dono, e alvos ausentes recebem `404`.

Repetir uma operação com o mesmo `Idempotency-Key`, corpo e rota usando UUID
ou nome reproduz a resposta do mesmo UUID sem repetir o envio. A autorização
atual é verificada antes do replay, inclusive após mudança de dono; após
renomear, o nome antigo retorna `404` e o novo compartilha o replay existente.

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances` | `{"name","external_ref"?,"owner_user_id"?,"webhook"?}` com `webhook:{url,enabled,events}` → `201` com `data.instance` (representação agregada abaixo) e `instance_api_key` em claro **uma única vez**; `external_ref` duplicada → `409`; acima da cota → `403 quota_exceeded`. O dono é a sessão criadora, ou o admin mais antigo (sobrescrevível por `owner_user_id` só global/admin) na criação por key global. |
| `GET /instances` | `200` com `{"data":{"items":[{"instance":...}]}}`, contendo todas as instâncias autorizadas na representação agregada abaixo, ordenadas por criação e id decrescentes. Contas `user` recebem só as próprias; instance key → `403`. |
| `GET /instances/stats` | `200` com `{total, by_status}`; query opcional `instance` aceita UUID ou nome. Instance key → `403`. |
| `GET /instances/{id}` | `200` com `{"data":{"instance":...}}` na representação agregada abaixo (nunca a key); `404` se não existir (id malformado também é `404`); instância de outro dono → `403`. |
| `PATCH /instances/{id}` | `{"name"?,"external_ref"?,"webhook"?}` com `webhook:{url,enabled,events}` → `200` com `data.instance` agregado; `external_ref` vazia limpa a referência; `url:""` limpa a URL e `events:[]` limpa a assinatura; webhook inválido → `422`. |
| `DELETE /instances/{id}` | `204`; encerra a sessão e apaga mensagens e mídias; operações seguintes → `404`. |
| `POST /instances/{id}/apikey/rotate` | Só global/admin → `200` com a nova key em claro uma vez; a antiga morre na hora. Serve de backfill para instâncias antigas sem key. |
| `DELETE /instances/{id}/apikey` | Só global/admin → `204`; a instância volta a responder só pela global até nova rotação. |
| `POST /instances/{id}/connect` | Inicia o pareamento → `200` com `{status, qr_code, qr_expires_at}`; instância já `connected` → `200` com `{status:"connected"}` sem QR (idempotente); já em `pairing` devolve o QR atual. |
| `GET /instances/{id}/qr` | `200` com o QR atual e a validade; reemite um QR novo quando o anterior expirou ou o pareamento ainda não começou; `409` se a sessão já está conectada. |
| `POST /instances/{id}/disconnect` | `204`; encerra a sessão, remove as credenciais e não reconecta. |
| `GET /instances/{id}/status` | `200` com `{status, whatsapp_jid, last_error, last_connected_at}`. |
| `POST /instances/{id}/numbers/check` | `{"phone"}` → `200` com `{exists, jid, normalized}`; número malformado/ausente do WhatsApp vem `exists:false`; sessão sem resolução confiável → `503`. |

**BREAKING:** `GET /instances` não usa paginação e remove `next_cursor` da
resposta. Clientes devem consumir `data.items` em uma única chamada; queries
antigas `limit` e `cursor` são ignoradas, inclusive valores inválidos. Uma
coleção grande produz uma resposta maior, sem limite oculto. Mensagens, grupos
e newsletters mantêm seus contratos de paginação.

**BREAKING:** `webhook` saiu da raiz de `instance` e existe só em
`integration.webhook`. `GET /instances`, `GET /instances/{id}`,
`POST /instances` e `PATCH /instances/{id}` devolvem também
`integration.chatwoot_config` (`null` sem configuração persistida) e
`settings`. `settings.default_disappearing` ecoa o último valor aceito por
`PUT /instances/{id}/chats/default-disappearing` (`0`, `24h`, `168h` ou
`2160h`) e é `null` quando nunca configurado. `settings.profile`,
`settings.privacy` e `settings.status_privacy` são objetos só quando
`connection.status` é `connected`; caso contrário, ou se a fonte falhar, o
bloco é `null` e a leitura da instância continua.

**BREAKING:** o bloco `webhook` saiu da raiz da instância e passou a
`integration.webhook` (o contrato cortado em 2026-10-06 o tinha na raiz; o
consumidor deve ler `instance.integration.webhook`). A representação pública
da instância — em `data.instance` e em `data.items[].instance`, inclusive nas
respostas de create e update — ficou:

```json
{"id": "...", "name": "...",
 "connection": {"status": "...", "last_error": {...}|null, "last_connected_at": "..."},
 "integration": {
   "webhook": {"enabled": false, "url": null, "events": ["message", "..."]},
   "chatwoot_config": {"is_enabled": false, "...": "..."} | null
 },
 "settings": {
   "default_disappearing": "0" | "24h" | "168h" | "2160h" | null,
   "profile": {"name": "...", "status_text": "...", "photo_url": "..."} | null,
   "privacy": {"last_seen": "...", "profile_photo": "...", "status": "...",
               "read_receipts": "...", "groups_add": "..."} | null,
   "status_privacy": {"mode": "...", "...": "..."} | null
 },
 "created_at": "...", "updated_at": "..."}
```

Semântica de `null` por bloco: `integration.chatwoot_config` é `null` quando
nunca houve configuração Chatwoot persistida (o `GET /instances/{id}/chatwoot`
próprio continua sintetizando um config desabilitado — o agregado não);
`settings.default_disappearing` ecoa o literal aceito pelo último
`PUT /instances/{id}/chats/default-disappearing` e é `null` enquanto nunca
configurado (distinto de `"0"` = desligado); os blocos vivos `profile`,
`privacy` e `status_privacy` só são buscados com `connection.status ==
"connected"` e vêm `null` nos demais estados ou quando a fonte falha — a
indisponibilidade de um bloco nunca derruba a leitura nem os demais itens da
listagem (montagem com concorrência limitada por requisição). Campos internos
seguem ocultos: `external_ref`, `owner_user_id`, `whatsapp_jid`/`device_jid`,
hashes e o token Chatwoot (write-only) não aparecem em nenhum bloco.

Estados de instância: `disconnected`, `pairing`, `connected`, `error`. Restrição
de conta vira `error` com motivo e **não** reconecta automaticamente; queda
transitória reconecta com espera crescente.

Cotas: teto global (`WZAP_MAX_INSTANCES`) + cota por usuário (ajustável em
`PATCH /users/{id}`); `0` = ilimitado. Toda instância conta, em qualquer
estado; acima da cota a criação responde `403 quota_exceeded`. Contas
`admin` (e a key global) bypassam ambas.

### Webhooks

Cada instância tem webhook próprio (`webhook_url`, `webhook_enabled`,
`webhook_events`, gerenciáveis no create/update por quem opera a
instância; nas leituras o bloco vive em `integration.webhook`). `events` aceita os 4 tipos padrão (`message`, `receipt`,
`connection`, `message.status`) mais os opt-in das novas famílias
(`poll.vote`, `message.reaction`, `interactive.response`,
`group.participants`, `group.info`, `call.offer`); omitir `webhook_events`
assina exatamente os 4 originais (os novos nunca entram no default).
Tipo desconhecido → `422`. Só URLs HTTP(S);
HTTP fora de loopback → `422` (fora de loopback use HTTPS). Sem URL, com o
webhook desabilitado ou sem instance key, nada é entregue.

A entrega é um POST JSON com o envelope versionado acrescido do `event` cru
(whatsmeow serializado; blobs acima do limite são cortados com a omissão
marcada, a mídia segue pela URL do envelope) e a instance key vigente no
header `apikey:` — sem HMAC, a confidencialidade depende de HTTPS. Falhas
(rede, timeout, status fora de 2xx) geram retries exponenciais limitados;
esgotados, a entrega é descartada como dead-letter em log e persistida na
tabela `webhook_dead_letters` (deduplicada por `event_id`, cauda limitada a
500 por instância, inspecionável via SQL) sem travar as
seguintes. A entrega é at-least-once: deduplique pelo `event_id` estável.

### Manager e Swagger

- Console web em `/manager` (inglês, sem credencial para carregar; sem
  sessão redireciona ao login): admin vê tudo + contas/cotas; `user` vê só
  as próprias instâncias (lista, criação com key exibida uma vez, edição,
  remoção com confirmação digitada, QR com polling, keys, webhook, envio de
  teste e mensagens). Build Nuxt estático embutido no binário.
- Documentação interativa em `/swagger/*` (Swagger 2.0), cobrindo cada
  método/rota explícito, incluindo Chatwoot e as entradas públicas do Manager.
  Informe uma key global/de instância em `Authorize` uma única vez: o Swagger
  envia o header `apikey:` automaticamente nas chamadas de máquina. As
  operações não pedem a key nem `X-Request-Id` em campos manuais; o serviço
  gera o identificador automaticamente e o retorna no header de resposta
  `X-Request-Id`. Rotas de autenticação dupla também aceitam o cookie
  `wzap_session`; o Swagger 2.0 não modela segurança por cookie.
- Respostas REST JSON normais usam `{"data": ...}` e erros padronizados
  usam `{"error": {"code", "message"}}`. As exceções estão nos schemas:
  `204` sem corpo, mídia binária, webhook Chatwoot com `{"content":""}` e
  Manager com HTML/redirecionamento ou `503` em texto simples. `/readyz`
  usa `data` tanto em `200` quanto em `503`.
- Regenere os três artefatos em `docs/` com a versão fixada abaixo; o CI
  falha com docs defasadas:

  ```sh
  go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs
  ```

### Mensagens

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances/{id}/messages/text` | `{"to","text"}` → `202` com `{"message_id","status":"queued"}`. |
| `POST /instances/{id}/messages/location` | `{"to","latitude","longitude"}` → `202`. |
| `POST /instances/{id}/messages/contact` | `{"to","display_name","vcard"}` → `202`. |
| `POST /instances/{id}/messages/media` | `multipart/form-data`: `to`, `type` (`image\|video\|audio\|document\|sticker`), `file` e opcionais `caption`, `filename`, `ptt` (`true`/`1`) → `202`. Tipo declarado precisa bater com o `Content-Type` do arquivo; `sticker` exige `image/webp`. |
| `POST /instances/{id}/messages` | `{"to","type":"poll\|reaction\|list\|buttons", ...}` → `202` com `{"message_id","status":"queued"}`. Tipos legados (`text`/`location`/`contact`) e `sticker` são `422` aqui (usem as rotas próprias). |
| `GET /instances/{id}/messages?limit&cursor` | `200` com `{"items":[...],"next_cursor"}`; `limit` padrão `50`, máximo `100`. |
| `GET /instances/{id}/messages/{message_id}` | `200` com estado atual, marcos temporais (`delivered_at`, `read_at`), tentativas (`attempts`) e `last_error`. |
| `GET /media/{id}` | Conteúdo bruto (fora do envelope) com `Content-Type`/`Content-Length` corretos; sem credencial → `401`; mídia inexistente ou expirada → `404`. |

O destinatário aceita telefone em formato livre: o serviço resolve o JID
canônico do WhatsApp aplicando a regra brasileira do 9º dígito antes de
enfileirar. Número inexistente → `422`; instância não conectada → `409`; mídia
inválida, tipo não suportado ou acima de `WZAP_MAX_MEDIA_BYTES` → `422`.

Os quatro POSTs de envio aceitam `Idempotency-Key` opcional (TTL de 24 h,
escopo por instância): repetir a mesma key com o mesmo conteúdo devolve a
resposta original com `X-Idempotent-Replay: true`; mesma key com conteúdo
diferente → `422`; enquanto a original corre → `409`. Sem a key o
comportamento é o envio normal.

Ciclo de vida da mensagem: `queued → sending → sent|failed`, com retries
exponenciais (até 5 falhas transitórias, teto de 2 min por espera) e retomada
de mensagens presas em `sending` por mais de 5 min. Recibos de entrega/leitura
atualizam `delivered_at`/`read_at` e geram o evento de recibo.

Tipos de mídia aceitos no upload: `image/jpeg`, `image/png`, `image/webp`,
`video/mp4`, `video/3gpp`, `audio/aac`, `audio/amr`, `audio/mpeg`, `audio/mp4`,
`audio/ogg`, `application/pdf`, `text/plain`, `.doc`, `.xls`, `.ppt`, `.docx`,
`.xlsx` e `.pptx`.

Limites das mensagens ricas (validados antes de enfileirar, `422` fora
deles, nada persistido): enquete com pergunta de 1..300 caracteres, 2..12
opções de 1..100 caracteres cada e `selectable_count` 0 ou 1 (omitido vira
1); lista com 1..10 seções de 1..10 linhas cada e textos de 1..300
caracteres; botões com 1..3 botões (`id`/`title` de até 64 caracteres, corpo
e rodapé de até 300). Reação com emoji vazio remove a reação anterior;
reações de saída endereçam mensagens enviadas pela própria instância. Sem
suporte ausente conhecido no upstream pinado: nenhuma rota emite `501` por
tipo não suportado (tudo construtível no whatsmeow atual).

### Ciclo de vida, presença e pareamento por telefone

Todas síncronas, direto na sessão (sem outbox, sem `202`): instância não
conectada → `409`; instância inexistente → `404`; sem ownership → `403`.

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances/{id}/messages/revoke` | `{"chat","message_id"}` → `200` com `{"revoked":true}`. Alvo inválido → `422` (nunca `500`); o campo `reason` existe `omitempty` para compatibilidade futura. O adapter não sinaliza "fora da janela": envio bem-sucedido ao protocolo é `revoked:true`; a janela do WhatsApp é documentada no Swagger. |
| `POST /instances/{id}/chats/mark-read` | `{"chat","message_id","sender"?}` → `200`. Em grupo `sender` (autor) é obrigatório (`422` sem ele); em conversa direta é completado com o próprio `chat` quando ausente. |
| `POST /instances/{id}/presence` | `{"chat","state"}` → `200`. Allowlist `composing\|paused` (conversa) e `available\|unavailable` (usuário); fora dela → `422` antes de tocar a sessão. Sem modo contínuo/heartbeat. |
| `POST /instances/{id}/pair-phone` | `{"phone"}` → `200` com `{pairing_code, expires_at}` (código de 8 dígitos). Exige canal de pareamento aberto (`connect` antes): já `connected` ou sem canal → `409` sem emitir código (a expiração é lida antes da emissão); número vazio/malformado → `422` genérica (anti-enumeração, sem logar o número). |

### Grupos

Operações síncronas de conta/conversa (sem outbox): `409` desconectada,
`404` grupo ou instância desconhecidos, `403` sem ownership **ou** sem
permissão no grupo, `422` conteúdo inválido. Nome de grupo limitado a 25
runas (limite do assunto no upstream).

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances/{id}/groups` | `{"name","participants"?}` → `201` com `{group, invite_code}`. Se o grupo for criado mas a leitura do convite falhar, a resposta continua `201` com o grupo e `invite_code` vazio (`omitempty`): **não** repita o create (duplicaria o grupo); reconcilie via `GET .../invite`. |
| `GET /instances/{id}/groups/{group_id}` | `200` com os metadados + `updated_at` (refresh sob demanda, ver abaixo). |
| `PATCH /instances/{id}/groups/{group_id}` | `{"name"?,"description"?}` → `200`. |
| `PUT /instances/{id}/groups/{group_id}/photo` | Corpo `image/*` cru → `200`; acima de `WZAP_MAX_MEDIA_BYTES` → `413`. Só troca (sem remoção). |
| `POST /instances/{id}/groups/{group_id}/participants` | `{"action":"add\|remove\|promote\|demote","participants":[...]}` → `200`. `leave` próprio é pela rota de saída; remover terceiros exige admin. |
| `GET /instances/{id}/groups/{group_id}/invite` | `200` com o código vigente. |
| `POST /instances/{id}/groups/{group_id}/invite/reset` | Revoga o código atual e emite outro → `200`. |
| `POST /instances/{id}/groups/join` | `{"code"}` ou link cheio → `200`. |
| `POST /instances/{id}/groups/{group_id}/leave` | Saída própria → `200`. |

Metadados (`group_metadata`, migration aditiva `00006`): toda leitura live
faz write-through do cache e expõe `updated_at`; o cache é log de refresh,
nunca fonte (sem leitura stale/TTL — consultar sempre reflete o upstream).
Falha de cache só loga e devolve o live; sem store configurado a leitura
passa direto (`updated_at` zero). `unfollow`/saída não tocam o cache.

### Newsletters

`409` desconectada, `404` canal desconhecido, `403` sem ownership.
Listagem paginada por cursor (`limit` padrão 50, máximo 100, `next_cursor`
com o último canal).

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances/{id}/newsletters/follow` | `{"channel"}` → `200` com a assinatura + `updated_at`. |
| `POST /instances/{id}/newsletters/unfollow` | `{"channel"}` → `200` (não toca o cache de metadados). |
| `GET /instances/{id}/newsletters/{channel}` | `200` com os metadados + `updated_at`. |
| `GET /instances/{id}/newsletters` | `200` com `{"items":[...],"next_cursor"}` ordenado por canal. |

### Status/stories e chamadas

Status publica sob `/instances/{id}/status/updates` (`GET /status` segue
sendo o connection-status). Publicar responde `202` + `message_id` com
replay `X-Idempotent-Replay` via o mesmo middleware de idempotência das
mensagens, mas o backing é **síncrono fire-and-forget** para o broadcast
(`status@broadcast`) — sem retry de outbox. Texto/caption de 1..700 runas;
mídia acima de `WZAP_MAX_MEDIA_BYTES` → `422`; `409` desconectada. A
 listagem cobre só status publicados desde o boot e poda entradas expiradas (~24 h do protocolo); apagar status
 desconhecido ou anterior ao boot → `404`.

| Método e rota | Corpo/Resposta |
| --- | --- |
| `POST /instances/{id}/status/updates` | `{"text"}` → `202` com `{"message_id","status":"published"}`. |
| `POST /instances/{id}/status/updates/media` | `multipart/form-data` imagem/vídeo + `caption` opcional → `202`. |
| `GET /instances/{id}/status/updates` | `200` com `{"items":[...]}` (array nunca nil). |
| `DELETE /instances/{id}/status/updates/{status_id}` | `200` com `{"deleted":true}`; desconhecido → `404`. |
| `POST /instances/{id}/calls/reject` | `{"call_id","from"}` → `200` com `{"rejected":true}`; campos ausentes → `422`; `409` desconectada; `501 not_supported` quando o upstream não suportar a rejeição. O companion nunca inicia chamada — só observa (evento `call.offer`) e rejeita. |

### Perfil e privacidade

Síncronas, `200`: `409` desconectada, `422` fora das allowlists (validado
antes da sessão; patch vazio → `422`).

| Método e rota | Corpo/Resposta |
| --- | --- |
| `GET /instances/{id}/profile` | `200` com nome, recado e URL da foto (push name best-effort). |
| `PATCH /instances/{id}/profile` | `{"name"?,"status_text"?}` → `200`. Nome 1..100 caracteres; recado 0..500 (vazio limpa). **Nome → `501 not_supported`** (a lib pinada não expõe o setter; quando `name` vem, nada é aplicado — nem o recado). |
| `PUT /instances/{id}/profile/photo` | Corpo `image/*` → `200`; acima do cap → `413`. **Foto → `501 not_supported`** (mesmo motivo). |
| `GET /instances/{id}/privacy` | `200` com a configuração vigente. |
| `PUT /instances/{id}/privacy` | `{"last_seen"?,"profile_photo"?,"status"?,"groups_add"?,"read_receipts"?}` → `200`. `last_seen`/`profile_photo`/`status`/`groups_add` em `all\|contacts\|contact_blacklist\|none`; `read_receipts` em `all\|none` (a lib modela receipts como switch de dois valores, não boolean). Todas as operações de privacidade são suportadas (sem `501`); `SetPrivacy` não é atômico — falha parcial pode aplicar um subset. |

## Chatwoot

Conector opcional que espelha mensagens do WhatsApp no Chatwoot em tempo real
e importa o histórico via SQL direto no Postgres do Chatwoot. Sem
`WZAP_CHATWOOT_IMPORT_DB_URL` o import fica inerte e o espelho segue normal.

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `WZAP_CHATWOOT_ENABLED` | `false` | Liga o conector (espelho, webhook, import). |
| `WZAP_CHATWOOT_BOT_CONTACT` | `123456` | Identificador do contato operacional (avisos de conexão e import). |
| `WZAP_CHATWOOT_MESSAGE_READ` | `false` | Projeta recibos de leitura no Chatwoot. |
| `WZAP_CHATWOOT_MESSAGE_DELETE` | `false` | Sincroniza revogações nos dois sentidos. |
| `WZAP_CHATWOOT_IMPORT_DB_URL` | vazio | URI do Postgres do Chatwoot para o import; vazia desliga o import. |
| `WZAP_CHATWOOT_IMPORT_PLACEHOLDER` | `false` | Mensagem sem conteúdo vira `(mídia não importada)` em vez de ser pulada. |
| `WZAP_CHATWOOT_TOKEN_KEY` | vazio | Chave base64 de 32 bytes que cifra os `token`s no banco (`enc:v1:` AES-256-GCM); vazia mantém texto claro com `warn` no boot. Malformada falha o boot. Tokens antigos são cifrados no boot (best-effort). Perder a chave exige recadastrar os tokens. |

| Método e rota | Corpo/Resposta |
| --- | --- |
| `PUT /instances/{id}/chatwoot` | Configuração do conector → `200`; validação falha → `422`. O `token` é aceito só na escrita e nunca volta nas respostas. **BREAKING:** `url` com `http` exige host loopback (`localhost`, `127.0.0.0/8`, `::1`); demais hosts exigem `https` para o token não trafegar em texto claro. |
| `GET /instances/{id}/chatwoot` | `200` com a configuração (sem o `token`) e a `webhook_url`. |
| `POST /instances/{id}/chatwoot/import` | Import manual → `202` com `{"imported":N}`, onde N conta mensagens importadas (contatos não entram na conta). |
| `POST /instances/{id}/chatwoot/command` | Comando operacional autenticado (dual auth) → `200` com `{"ok":true}`. Corpo `{"command":"status\|init[:number]\|clearcache\|disconnect","conversation_id":N}`; confirma na conversa. O webhook aberto nunca executa comandos. |
| `POST /chatwoot/webhook/{id}` | Webhook aberto por desenho (sem auth), responde corpo de bot. |

O espelho cobre texto, mídias (imagem, vídeo, áudio, documento e figurinha
via anexo), contato simples e em lista, localização, listas, reactions,
botões interativos (incluindo PIX), pedidos, produtos e anúncios (com a
miniatura anexada quando os bytes vêm no evento). Enquetes, chamadas, avisos
de protocolo e reactions criptografadas não têm equivalente em texto e são
puladas com `warn`, sem derrubar o worker.

O import ordena por telefone+tempo, deduplica por `source_id` (`WAID:`,
compartilhado com o espelho), respeita `days_limit` e dispara em três
gatilhos: automático pós-pareamento (uma vez), manual (`POST .../import`) e
cron de 30 min com janela de 6 h (limpa acumuladores e o cache do conector);
falha num lote retorna a contagem parcial junto com o erro; início e
resultado avisam na conversa operacional em pt-BR.

Riscos operacionais: token guardado em claro mas nunca ecoado (só escrita),
webhook aberto por desenho com busca de anexos sob allowlist SSRF
(só `http`/`https`, no máximo 3 redirects, metadados/link-local sempre
bloqueados, IP privado/loopback só para o host do Chatwoot configurado) e
SQL direto no banco do Chatwoot (frágil a upgrades — módulo isolado,
desligável pela URI; `display_id` com retry limitado em conflito de unicidade
contra escritas do Rails).

## Eventos

O serviço publica em um stream JetStream (nome em `WZAP_NATS_STREAM`, padrão
`WZAP`, subjects `wzap.>`, retenção em `WZAP_EVENT_RETENTION_DAYS`). Cada
instância tem doze subjects:

- `wzap.instances.{instance_id}.message` — mensagem recebida.
- `wzap.instances.{instance_id}.receipt` — recibo de entrega/leitura/reprodução.
- `wzap.instances.{instance_id}.connection` — mudança de estado da instância.
- `wzap.instances.{instance_id}.message.status` — status de envio.
- `wzap.instances.{instance_id}.message.edit` — edição de mensagem recebida.
- `wzap.instances.{instance_id}.message.delete` — remoção/revoke de mensagem recebida.
- `wzap.instances.{instance_id}.message.poll.vote` — voto em enquete (`type: poll.vote`).
- `wzap.instances.{instance_id}.message.reaction` — reação recebida, incl. remoção (`type: message.reaction`).
- `wzap.instances.{instance_id}.message.interactive.response` — resposta de lista/botão/fluxo (`type: interactive.response`).
- `wzap.instances.{instance_id}.group.participants` — entradas/saídas (`type: group.participants`).
- `wzap.instances.{instance_id}.group.info` — assunto/tópico/foto (`type: group.info`).
- `wzap.instances.{instance_id}.call.offer` — oferta/aceite/recusa/fim de chamada (`type: call.offer`, campo `state`).

O webhook por instância assina os quatro tipos padrão (`message`, `receipt`,
`connection`, `message.status`); os demais (`poll.vote`,
`message.reaction`, `interactive.response`, `group.participants`,
`group.info`, `call.offer`) são opt-in via `webhook_events`, e
edição/remoção trafegam só no NATS.

Todo evento carrega o mesmo envelope versionado (`event_version: 1`);
`event_id` é estável e enviado como `Nats-Msg-Id` para deduplicação no broker
(janela de 2 min). A publicação passa pelo outbox (`event_outbox`) e por um
relay com retry: o evento sobrevive a reinícios e a indisponibilidades do
broker. A entrega é **at-least-once**: deduplique pelo `event_id` e trate
reentregas como idempotentes.

```json
{
  "event_id": "0f8c3f1e-...",
  "event_version": 1,
  "type": "message",
  "instance_id": "3b1d...",
  "occurred_at": "2026-09-13T18:00:00.123456789Z",
  "payload": { "from_jid": "...", "chat_jid": "...", "is_group": false,
               "message_id": "...", "timestamp": "...", "type": "text",
               "text": "olá" }
}
```

Payloads por `type`:

- `message`: `from_jid`, `chat_jid`, `is_group`, `message_id`, `timestamp`,
  `type` (tipo do WhatsApp: `text`, `image`, `video`, `audio`, `document`,
  `location` etc.), `text` quando houver. Com mídia armazenada:
  `media: {media_id, mimetype, filename?, size, url, expires_at}`; sem mídia
  (acima do limite, indisponível ou falha no download):
  `media_omitted: {reason}`.
- `receipt`: `message_ids` (ids WhatsApp das mensagens atualizadas), `status`
  (`delivered`, `read` ou `played`), `chat_jid`, `timestamp`. Correlacione com
  `whatsapp_id` dos eventos `message.status`.
- `connection`: `status` (`disconnected`, `pairing`, `connected`, `error`),
  `whatsapp_jid` quando conhecido e `reason` em falha.
- `message.status`: `message_id` (UUID do wzap), `status` (`sent` ou `failed`),
  `whatsapp_id` quando enviada e `error` quando falha.
- `poll.vote`: `from_jid`, `chat_jid`, `is_group`, `poll_message_id`,
  `selected_option_ids` (hex dos hashes do protocolo) e
  `selected_option_names` (vazio quando o fio carrega só hashes —
  irreversíveis — ou o voto não pôde ser descriptografado; o evento é
  emitido mesmo assim com eleitor + chave da enquete).
- `message.reaction`: `from_jid`, `chat_jid`, `is_group`,
  `target_message_id`, `emoji` (vazio = remoção).
- `interactive.response`: `from_jid`, `chat_jid`, `is_group`, `message_id`,
  `source` (`buttons`|`list`|`native_flow`), `selected_id`, `title`.
- `group.participants`: `group_jid`, `actor_jid` (vazio quando o upstream
  não informa), `affected` (vazio = a própria instância).
- `group.info`: `group_jid` + snapshot `name`/`description`.
- `call.offer`: `call_id`, `from_jid`, `state`
  (`offer`|`accept`|`reject`|`end`), `is_video` (best-effort).

```json
{
  "event_id": "0f8c3f1e-...",
  "event_version": 1,
  "type": "message.reaction",
  "instance_id": "3b1d...",
  "occurred_at": "2026-09-13T18:00:00.123456789Z",
  "payload": { "from_jid": "...", "chat_jid": "...", "is_group": false,
               "target_message_id": "...", "emoji": "👍",
               "timestamp": "2026-09-13T18:00:00Z" }
}
```

## Operação local

O `docker-compose.yml` da raiz sobe o serviço em `127.0.0.1:8081`, com volume
próprio e dependências Postgres/NATS.

```bash
# na raiz do repositório
docker compose up -d wzap

# prontidão
curl 127.0.0.1:8081/readyz
# {"data":{"status":"ready","checks":{"migrations":"ok","nats":"ok","postgres":"ok"}}}
```

O compose define `WZAP_API_KEY=${WZAP_API_KEY:-dev-wzap-token}`
(sobrescreva definindo a variável no ambiente ou no `.env` da raiz),
`WZAP_DATABASE_URL` apontando para o banco `wzap` dentro da rede,
`WZAP_NATS_URL=nats://nats:4222`, `WZAP_PUBLIC_URL=http://127.0.0.1:8081` e
`WZAP_AUTO_MIGRATE=true`. O `WZAP_PUBLIC_URL` é o que faz `media.url` dos
eventos apontar para um endereço alcançável pelas aplicações consumidoras;
ajuste-o no compose ou com um override quando necessário. Para o seed do
admin inicial, defina também `WZAP_ADMIN_EMAIL` e `WZAP_ADMIN_PASSWORD`.

**Banco em volume pré-existente:** o script `docker/postgres-init.sql` cria o
banco de teste automaticamente apenas quando o volume do Postgres é
inicializado do zero. Em um volume existente, crie-o uma única vez:

```bash
docker compose exec postgres createdb -U wzap wzap_test
```

```bash
# smoke test autenticado (rotas na raiz, header apikey:)
curl -sS -X POST 127.0.0.1:8081/instances \
  -H 'apikey: dev-wzap-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"smoke","external_ref":"smoke-1"}'
# {"data":{"id":"...","status":"disconnected","owner_user_id":"...","instance_api_key":"..."}}

curl -sS -X POST 127.0.0.1:8081/instances/<id>/connect \
  -H 'apikey: dev-wzap-token'
# {"data":{"status":"pairing","qr_code":"2@...","qr_expires_at":"..."}}
```

Encerramento: o serviço trata `SIGINT`/`SIGTERM`, drena as requisições em voo e
só depois para, nesta ordem: outbox de mensagens, limpeza de mídia, worker de
webhooks, mirror Chatwoot (se habilitado), agendador de import Chatwoot (se
habilitado) e, por último, o relay — que publica os eventos pendentes do
outbox. Todo o encerramento compartilha um limite de 10 s e um segundo sinal
o aborta imediatamente.

Eventos e webhooks: a fila de outbox persiste antes do broker; o relay publica
do Postgres. Webhooks por instância só disparam **depois** que o outbox
grava o envelope com sucesso. A entrega HTTP é best-effort (retries e dead
letters); se a fila interna encher, eventos podem ser descartados com contador
de log — o outbox/NATS permanece a fonte durável.

## Testes

Os testes de integração exigem um Postgres acessível cujo banco termine em
`_test`; sem `WZAP_TEST_DATABASE_URL` eles são pulados. Crie o banco de teste
uma vez:

```bash
docker compose exec postgres createdb -U wzap wzap_test
```

```bash
# suite completa
WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' \
  go test ./... -count=1

go vet ./...
golangci-lint run
go build ./...
gofmt -l .          # saída vazia = formatado
```

No CI (`.github/workflows/ci.yml`) rodam `go vet`,
`golangci-lint` v2.13.2, `go test ./...` e `go build ./...` com um Postgres de
serviço.

## Checklist manual de pareamento e envio real

Depende de um aparelho humano com WhatsApp; execute uma vez por release ou
quando o fluxo de sessão mudar. Ainda **não executado** (pendente de número de
teste):

- [ ] Subir o serviço (`docker compose up -d wzap`) e confirmar `/readyz` com
      `postgres`, `migrations` e `nats` em `ok`.
- [ ] Criar uma instância de teste e iniciar o pareamento; renderizar a string
      `qr_code` em um gerador de QR e escanear com o aparelho.
- [ ] Confirmar `GET .../status` em `connected` com `whatsapp_jid` e o evento
      `...connection` publicado.
- [ ] Enviar texto, localização, contato e uma mídia para um número de teste;
      confirmar `202`, estado `sent` e os recibos `delivered`/`read`.
- [ ] Receber uma mensagem de texto no aparelho e confirmar o evento
      `...message` com o payload.
- [ ] Receber uma mídia, confirmar o evento com `media` e baixar por
      `GET /media/{id}` com a key (e confirmar `401` sem credencial).
- [ ] Desconectar a instância, confirmar que não há reconexão e que
      `DELETE` responde `204` com `404` nas operações seguintes.

## Referências

- Specs e decisões: `openspec/changes/wzap-foundation/` (proposal, design,
  specs por capability) e `openspec/changes/wzap-product/` (contrato produto:
  contas, api keys, webhooks, manager; quebras marcadas **BREAKING**).
- Cobertura WhatsApp (revogação, leitura, presença, pair-phone, mensagens
  ricas, grupos, newsletters, status, chamadas, perfil/privacidade):
  `openspec/specs/wzap-{message-lifecycle,presence,phone-pairing,rich-messaging,groups,newsletters,status-calls,profile-privacy}/`
  e `openspec/changes/expand-whatsapp-coverage/` (proposal, design, specs por
  capability; sem **BREAKING** — tudo aditivo).
- Licenças e trechos reaproveitados: `THIRD_PARTY_NOTICES.md`.
