# wzap-chatwoot-config Specification

## Purpose

Configuração do conector Chatwoot por instância, mais chaves globais do processo, com validação explícita e segredos sob responsabilidade operacional.

## Requirements

### Requirement: Configuração por instância

Cada instância SHALL ter configuração própria gerenciável por quem pode operá-la. **BREAKING**: os nomes curtos aprovados são is_enabled, url, account_id, token, inbox_name, is_sign_enabled, sign_delimiter, is_reopen_enabled, is_pending_enabled, is_merge_enabled, is_import_contacts, is_import_messages, import_days, is_auto_create, organization, logo e ignored_jids. Ao habilitar, URL/account/token válidos SHALL ser exigidos; flags não booleanas MUST ser rejeitadas com 422 sem alterar a configuração anterior. HTTP SHALL exigir host loopback; demais hosts SHALL exigir HTTPS. Inbox vazia SHALL assumir o identificador da instância e delimitador vazio SHALL assumir quebra de linha. Com WZAP_CHATWOOT_TOKEN_KEY configurada, o token SHALL manter a proteção AES-256-GCM existente e o backfill de tokens legados; sem a chave, SHALL conservar o comportamento e aviso existentes. Token MUST NOT ser devolvido nas respostas.

#### Scenario: Habilitar conector

- **WHEN** o operador grava configuração válida com is_enabled true
- **THEN** o espelho e o webhook da instância passam a operar

#### Scenario: Config inválida

- **WHEN** o operador grava URL malformada ou omite account_id/token ao habilitar
- **THEN** recebe 422 e a configuração anterior é mantida

#### Scenario: Leitura sem configuração

- **WHEN** a instância nunca foi configurada
- **THEN** a leitura informa integração desligada sem expor token

### Requirement: Chaves globais

O processo SHALL expor `WZAP_CHATWOOT_ENABLED` (gate global; desligado bloqueia `set`/`find`/espelho/webhook com `400`), `WZAP_CHATWOOT_IMPORT_DB_URL` (URI do Postgres do Chatwoot; ausente desliga import e labels sem desligar o espelho), `WZAP_CHATWOOT_BOT_CONTACT`, `WZAP_CHATWOOT_MESSAGE_READ` e `WZAP_CHATWOOT_MESSAGE_DELETE`.

#### Scenario: Gate global desligado

- **WHEN** `WZAP_CHATWOOT_ENABLED` não é `true` e chega qualquer operação do conector
- **THEN** a operação é rejeitada com `400` e nada é executado

#### Scenario: Sem URI de import

- **WHEN** a URI do Postgres do Chatwoot está ausente e o espelho opera
- **THEN** mensagens espelham normalmente; só import e labels ficam indisponíveis com `warn` em log

### Requirement: Auto-criação de inbox

Com is_auto_create true, a gravação SHALL garantir inbox api com o webhook_url da instância, reusando por nome, o contato operacional e a mensagem init[:number]. A resposta SHALL incluir o webhook_url a cadastrar no Chatwoot.

#### Scenario: Auto-criação

- **WHEN** o operador grava com is_auto_create true e a inbox não existe
- **THEN** inbox, contato operacional e init passam a existir e a resposta informa o webhook_url
