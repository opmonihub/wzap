## Why

Uma sessão recém-pareada nesta máquina produziu dois `500` e um `404` que o cliente não consegue distinguir de falha interna, e o JetStream subiu em diretório temporário apesar do volume já montado. Os eventos duráveis se perdem no restart do broker, e o console trata foto inválida e perfil próprio como erro do servidor.

## What Changes

- O JetStream do compose local passa a gravar no volume já montado em `/data`, não em `/tmp`.
- `PUT /instances/{id}/groups/{group_id}/photo` com bytes que não são imagem responde `422`, sem alterar a foto. O `Content-Type` que não é `image/*` e o cap de tamanho permanecem `422` e `413`.
- `GET /instances/{id}/profile` de instância conectada responde `200` com o push name. O JID próprio com sufixo de dispositivo não derruba a leitura. Recado e URL da foto vêm quando o upstream os tem; ausência deles não vira `500`.
- A busca de grupo no console com JID vazio não chama a API. Hoje isso vira `GET .../groups/` e `404`.

## Out of Scope

- `GET /auth/me` `401` antes do login.
- Fechamento de websocket com `EOF` no código 515 logo após o pareamento: a sessão autenticou em seguida. É o restart de stream da biblioteca.
- Aviso `duplicate contacts` no history sync da biblioteca.
- `relation "goose_db_version" does not exist` na primeira sonda do goose, antes da tabela existir. O boot seguinte já estava na versão 6.
- Aviso de locale e `trust` no socket local da imagem Postgres Alpine.
- `GET /instances/{id}/chatwoot` com `400 chatwoot_disabled` quando `WZAP_CHATWOOT_ENABLED` não é `true`. O spec `wzap-chatwoot-config` exige esse `400`, e o console já mostra o conector desligado.
- Redação de telefone nos logs `info` emitidos pela biblioteca.
- Redirect genérico de barra final em todas as rotas.

## Capabilities

### New Capabilities

- Nenhuma.

### Modified Capabilities

- `wzap-operations`: o broker local persiste o JetStream no volume, não em diretório temporário.
- `wzap-groups`: bytes que não são imagem na troca de foto respondem `422`.
- `wzap-profile-privacy`: a consulta do perfil próprio de instância conectada responde `200` com push name, mesmo quando o JID armazenado traz sufixo de dispositivo.
- `wzap-manager`: a seção de grupos não dispara busca com JID vazio.

## Impact

- `docker-compose.yml` e `docker-compose.dev.yml` (comando do NATS). Recriar o broker descarta o store que hoje está em `/tmp` dentro do container; o outbox no Postgres continua a fonte do que ainda não foi publicado.
- Sessão whatsmeow (consulta de perfil próprio e classificação do erro de foto de grupo) e o handler HTTP que mapeia esse erro.
- Console: `GroupDetail` deixa de chamar `GET .../groups/` com identificador vazio.
- Sem mudança de envelope, de `event_version` ou de subject.
