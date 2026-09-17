# wzap-manager Specification

## Purpose

Console web do produto: quem instala administra tudo, cada cliente opera as
próprias instâncias (incluindo parear o próprio celular), sem tocar em
terminal ou REST manual.

## Requirements

### Requirement: Servido pelo próprio serviço

O console SHALL ser servido pelo serviço em `/manager`, em inglês, sem exigir
rede externa em runtime para seus próprios assets. Rotas internas do console
MUST resolver para o aplicativo (navegação direta e refresh funcionam).

#### Scenario: Acesso direto a rota interna

- **WHEN** o navegador abre diretamente uma rota interna do console
- **THEN** o aplicativo carrega e exibe a tela correspondente

### Requirement: Login e visões

O acesso SHALL exigir login por email/senha; sem sessão válida, qualquer tela
MUST redirecionar ao login. Contas `admin` SHALL ver a visão de operador
(todas as instâncias, contas, cotas); contas `user` SHALL ver somente as
próprias instâncias, com ações fora do escopo ausentes ou desabilitadas.

#### Scenario: Login de operador

- **WHEN** uma conta `admin` entra com credenciais válidas
- **THEN** vê todas as instâncias e a gerência de contas

#### Scenario: Login de cliente

- **WHEN** uma conta `user` entra com credenciais válidas
- **THEN** vê somente as próprias instâncias, sem gerência de contas

#### Scenario: Sessão ausente

- **WHEN** um visitante sem sessão abre qualquer tela
- **THEN** é redirecionado ao login

### Requirement: Gestão de instâncias

O console SHALL apresentar as instâncias em tabela padronizada com ordenação,
busca e paginação client-side, mantendo criar, editar, desconectar e remover
instâncias conforme o escopo da conta. A remoção SHALL exigir confirmação
digitada com o nome da instância.

#### Scenario: Localizar instância na tabela

- **WHEN** a conta digita parte do nome ou da referência externa na busca da tabela
- **THEN** a tabela exibe somente as instâncias carregadas que correspondem, sem nova chamada à API

#### Scenario: Ordenar instâncias

- **WHEN** a conta aciona a ordenação em uma coluna ordenável (nome ou estado)
- **THEN** as linhas carregadas são reordenadas de forma crescente/decrescente

#### Scenario: Paginar instâncias

- **WHEN** a conta navega entre páginas da tabela
- **THEN** vê uma fatia dos registros carregados com a posição atual indicada, sem perder o filtro aplicado

#### Scenario: Criar instância

- **WHEN** a conta cria uma instância dentro da cota
- **THEN** a instância aparece na tabela como `disconnected` com sua key exibida
  uma vez para cópia

#### Scenario: Remoção com confirmação

- **WHEN** a conta remove uma instância e digita o nome corretamente
- **THEN** a instância é removida; com nome divergente, nada acontece

#### Scenario: Cota excedida

- **WHEN** a conta tenta criar acima da cota
- **THEN** o console exibe o erro de cota sem criar nada

### Requirement: Gestão de contas em tabela

O console SHALL apresentar as contas em tabela padronizada com ordenação,
busca e paginação client-side, mantendo criar conta, editar cota e remover
conta (só admin). Quotas `0` SHALL significar ilimitado; remover dono com
instâncias SHALL responder `409` sem remover nada.

#### Scenario: Localizar conta na tabela

- **WHEN** o admin digita parte do email na busca da tabela
- **THEN** a tabela exibe somente as contas que correspondem, sem nova chamada à API

#### Scenario: Ordenar contas

- **WHEN** o admin aciona a ordenação em uma coluna ordenável (email, papel ou cota)
- **THEN** as linhas são reordenadas de forma crescente/decrescente

#### Scenario: Editar cota a partir da tabela

- **WHEN** o admin edita a cota de uma conta pela ação da linha
- **THEN** a linha reflete a nova cota e um aviso de sucesso é exibido

#### Scenario: Remover conta com instâncias

- **WHEN** o admin tenta remover uma conta que ainda possui instâncias
- **THEN** o console exibe o erro de conflito sem remover a conta

### Requirement: Estados e escopos das tabelas

As tabelas SHALL preservar os escopos por papel (admin vê tudo, `user` vê só as
próprias instâncias e não vê a gerência de contas), a coluna de dono visível
só para admin, e os estados de carregamento, falha com retry e vazio, em inglês.

#### Scenario: Visão do cliente

- **WHEN** uma conta `user` abre as instâncias
- **THEN** vê somente as próprias instâncias, sem a coluna de dono

#### Scenario: Falha de carregamento

- **WHEN** o carregamento da lista falha
- **THEN** a tabela exibe o erro com ação de retry que recarrega os dados

### Requirement: Pareamento com QR

O console SHALL exibir o QR de pareamento renderizado visualmente, com validade
e atualização automática (reemissão ao expirar e transição ao conectar), por
instância no escopo da conta.

#### Scenario: Parear pelo console

- **WHEN** a conta abre o pareamento de uma instância desconectada
- **THEN** vê o QR válido e, após a leitura no celular, o estado exibido vira
  `connected`

#### Scenario: QR expira na tela

- **WHEN** o QR exibido expira antes da leitura
- **THEN** um novo QR é exibido automaticamente sem ação manual

### Requirement: Keys e webhook

O console SHALL permitir copiar a key exibida uma vez na criação, rotacionar e
revogar a key (escopo admin/global na API, refletido na UI), e configurar
`url`/`enabled` do webhook por instância. Keys em claro MUST NOT ser reexibidas
após a primeira exibição.

#### Scenario: Rotacionar key

- **WHEN** a conta rotaciona a key de uma instância
- **THEN** a nova key é exibida uma vez para cópia e a antiga deixa de valer

#### Scenario: Configurar webhook

- **WHEN** a conta salva `url` válida, escolhe os tipos de evento e habilita o
  webhook
- **THEN** a configuração persiste e passa a valer para os próximos eventos

#### Scenario: Webhook com URL insegura

- **WHEN** a conta tenta salvar URL HTTP fora de loopback
- **THEN** o console exibe o erro de validação sem salvar

### Requirement: Envio de teste e mensagens

O console SHALL permitir verificar número, enviar mensagem de teste (texto e
mídia) e acompanhar o estado até `sent`/`failed`, além de consultar mensagens
da instância, tudo no escopo da conta.

#### Scenario: Envio de teste com acompanhamento

- **WHEN** a conta envia um texto de teste para um número válido
- **THEN** acompanha o estado até o desfecho `sent` ou `failed` com motivo

#### Scenario: Número inexistente

- **WHEN** a conta verifica ou envia para número inexistente no WhatsApp
- **THEN** o console exibe a falha sem enfileirar envio

### Requirement: Overview Home com métricas reais

O overview SHALL exibir stats por status (total, connected, disconnected/pairing, error), gráfico de criação por período e as 5 instâncias mais recentes, derivados de `GET /instances/stats` com fallback para contagem local da listagem quando o endpoint falhar.

#### Scenario: Stats do endpoint

- **WHEN** a conta abre o overview com `GET /instances/stats` saudável
- **THEN** vê total e por-status iguais aos da API, cada card ligando para `/instances`

#### Scenario: Fallback local

- **WHEN** `GET /instances/stats` falha
- **THEN** o overview deriva os mesmos cards da listagem cursor-acumulada e exibe aviso discreto

#### Scenario: Gráfico por período

- **WHEN** a conta troca o período (dia/semana/mês)
- **THEN** o gráfico reagrupa os `created_at` carregados sem nova chamada de série temporal

### Requirement: Endpoint aditivo de stats

`GET /instances/stats` SHALL responder `{"data":{"total":N,"by_status":{"connected":N,"disconnected":N,"pairing":N,"error":N}}}` respeitando o escopo da sessão (admin vê tudo, user só as próprias); instance keys SHALL receber 403 como nas demais rotas de coleção.

#### Scenario: Stats do operador

- **WHEN** um admin chama `GET /instances/stats`
- **THEN** recebe a contagem de todas as instâncias por status

#### Scenario: Stats do cliente

- **WHEN** um user chama `GET /instances/stats`
- **THEN** recebe a contagem só das instâncias que possui

#### Scenario: Chave de instância

- **WHEN** uma instance key chama `GET /instances/stats`
- **THEN** recebe 403 `forbidden`

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
