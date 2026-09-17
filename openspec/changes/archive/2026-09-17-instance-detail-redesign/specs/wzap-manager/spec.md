## MODIFIED Requirements

### Requirement: Listas e detalhe no padrão do template

As listas SHALL manter dados, filtros, ordenação, seleção e paginação atuais adotando o `ui` de tabela do template customers (bordas arredondadas, header com fundo) e footer com contagem de selecionados + paginação; o detalhe SHALL adotar header + faixa de stats + navegação por seções via `UNavigationMenu` horizontal no toolbar (padrão `settings.vue`) com 7 seções (overview, messages, groups, channels, profile, integrations, settings), mantendo os fluxos atuais de pairing, nome, key, webhook, envio e danger; a seção messages SHALL usar o split inbox full-width (lista selecionável + painel de detalhe + composer no rodapé, `USlideover` no mobile) e as demais seções SHALL centralizar forms em `lg:max-w-2xl`.

#### Scenario: Lista preservada

- **WHEN** a conta usa busca, filtro de status, ordenação, seleção ou paginação
- **THEN** o comportamento é o atual, só o visual segue o template

#### Scenario: Detalhe preservado

- **WHEN** a conta abre o detalhe
- **THEN** vê pairing, nome, key, webhook, test-send, messages e danger com os mesmos fluxos, reorganizados em header + stats + 7 seções (overview, messages, groups, channels, profile, integrations, settings)

#### Scenario: Messages em split inbox

- **WHEN** a conta abre a seção messages no desktop
- **THEN** vê a lista selecionável à esquerda e o detalhe com composer à direita; no mobile o detalhe abre em `USlideover`

## ADDED Requirements

### Requirement: Faixa de stats do detalhe

O detalhe SHALL exibir 4 cards subtle (padrão `HomeStats.vue`) — Connection (status/JID/last_connected), Identity (created/updated/external_ref), Webhook (enabled + contagem de inscritos), Chatwoot (enabled/disabled ou aviso de global-off) — sem contagens inventadas, derivados da instância carregada e da config Chatwoot.

#### Scenario: Stats após carregar

- **WHEN** a conta abre o detalhe de uma instância existente
- **THEN** os 4 cards refletem status/JID, datas/external_ref, webhook e Chatwoot da instância

#### Scenario: Chatwoot globalmente desligado

- **WHEN** o backend responde `chatwoot_disabled` (400) ao consultar a config
- **THEN** o card Chatwoot exibe aviso informativo e os forms da seção integrations ficam desabilitados

### Requirement: Seção overview

A seção overview SHALL reunir info da instância (JID, last_error, created/updated, external_ref), `PairingCard` com o contrato atual, card irmão de pair-phone (código de 8 dígitos + expiração, exigindo canal Connect prévio), form de rename e danger delete; desconectadas SHALL exibir o aviso `notConnected` e desabilitar ações que tocam a sessão.

#### Scenario: Pair-phone após Connect

- **WHEN** a conta abre o canal Connect e depois solicita o código pair-phone
- **THEN** vê o código de 8 dígitos com a expiração (`expires_at`) sem expor o telefone em logs

#### Scenario: Pair-phone sem canal

- **WHEN** a conta solicita pair-phone sem canal Connect aberto
- **THEN** o console exibe o erro de conflito (409) e orienta a conectar primeiro

### Requirement: Seção messages completa

A seção messages SHALL permitir composer estendido (text/media/location/contact/poll/reaction/list/buttons em sub-abas), consulta de histórico com cursores, detalhe com meta + attempts/IDs + `last_error`, e linha de ações revoke/mark-read/presence; envios SHALL usar `Idempotency-Key` fresca (`crypto.randomUUID()`) por clique com polling até `sent`/`failed`; mensagens 422 SHALL exibir o texto do servidor verbatim; instância desconectada SHALL bloquear envios com o aviso `notConnected`.

#### Scenario: Envio rico com acompanhamento

- **WHEN** a conta envia location, contact, poll, reaction, list ou buttons para número válido em instância conectada
- **THEN** o envio é aceito (202) e o estado é acompanhado até `sent` ou `failed` com motivo

#### Scenario: Revoke e mark-read

- **WHEN** a conta revoga uma mensagem enviada ou marca um chat como lido (com sender em grupos)
- **THEN** a ação é confirmada e o histórico reflete o estado resultante

#### Scenario: Download de mídia

- **WHEN** a conta solicita o download de uma mídia da instância
- **THEN** o conteúdo é baixado via blob URL sem exibir tokens ou chaves

### Requirement: Seção groups

A seção groups SHALL listar conversas em linhas estilo WhatsApp sobre primitivas do template (avatar, nome truncado, sub-linha com contagem de participantes + descrição truncada — nunca última mensagem inventada), com busca por JID/nome, seleção (`border-primary bg-primary/10`), painel/modal de detalhe (update/description/invite/photo/participants/leave) e CTAs de create + join-by-invite em `UModal`; create SHALL validar nome ≤25 caracteres; create 201 com `invite_code` vazio SHALL exibir reconciliação via GET invite e MUST NOT re-tentar create.

#### Scenario: Criar grupo e reconciliar convite

- **WHEN** a conta cria um grupo válido e a resposta traz `invite_code` vazio (lookup pós-create falhou)
- **THEN** o console exibe o grupo criado e orienta a reconciliar o código via GET invite sem duplicar o grupo

#### Scenario: Gerenciar participantes e foto

- **WHEN** a conta adiciona/remove/promove/rebaixa participantes ou envia foto (`Content-Type: image/*` octet-stream)
- **THEN** a operação é aplicada e o detalhe do grupo reflete o novo estado

### Requirement: Seção channels (newsletters + statuses)

A seção channels SHALL trazer newsletters (follow/unfollow + lookup + lista de seguidos com título/canal + contagem de seguidores) e statuses/stories (publish text/media + lista própria renderizada como bolhas só onde há texto/caption + delete) nas mesmas linhas de conversa; publish de status text SHALL validar 1..700 caracteres e kind image|video para mídia; publish SHALL ser fire-and-forget (sem retry de outbox).

#### Scenario: Seguir e consultar newsletter

- **WHEN** a conta segue um canal válido
- **THEN** o canal aparece na lista de seguidos com título e contagem de seguidores

#### Scenario: Publicar e remover status

- **WHEN** a conta publica um status de texto válido e depois o remove
- **THEN** o status aparece na lista própria e some após a remoção

### Requirement: Seção profile (perfil, privacidade, dispositivo)

A seção profile SHALL trazer profile get/patch/photo, privacy get/put, presence e reject-call, cada um em `UPageCard subtle`; profile name SHALL validar 1..100 e recado ≤500; privacy SHALL validar allowlists (`all|contacts|contact_blacklist|none`, receipts `all|none`) com ao menos um campo no patch; casos `501 not_supported` (nome/foto de perfil, reject-call em alguns upstreams) SHALL exibir aviso explícito de não-suportado.

#### Scenario: Atualizar perfil e privacidade

- **WHEN** a conta salva nome/recado válidos e um patch de privacidade com valores da allowlist
- **THEN** o perfil e a privacidade refletem os novos valores

#### Scenario: Recurso não suportado pelo upstream

- **WHEN** o backend responde `501 not_supported` a nome/foto de perfil ou reject-call
- **THEN** o console exibe o aviso de não-suportado sem alterar o estado exibido

### Requirement: Seções integrations e settings

A seção integrations SHALL mover o `WebhookCard` atual e somar Chatwoot config get/put (token write-only, GET mascara), import (`202 {"imported":N}` com contagem parcial + erro quando houver) e command, além da URL de webhook computada; a seção settings SHALL concentrar key rotate/revoke admin-gated (`OneTimeKeyDisplay`, sem reexibir em claro) + danger delete (rename fica no overview); não-admin MUST NOT ver o gerenciamento de keys.

#### Scenario: Import Chatwoot parcial

- **WHEN** a conta executa o import e o backend responde contagem parcial com erro
- **THEN** o console exibe `imported:N` junto ao erro sem ocultar o progresso

#### Scenario: Visão não-admin

- **WHEN** uma conta `user` abre o detalhe
- **THEN** a seção settings oculta o gerenciamento de keys e mantém o danger conforme o escopo
