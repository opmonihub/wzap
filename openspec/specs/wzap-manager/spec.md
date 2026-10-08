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

O console SHALL apresentar as instâncias em tabela padronizada com ordenação, busca e paginação client-side, mantendo criar, editar, desconectar e remover conforme o escopo da conta. A remoção SHALL exigir confirmação digitada com o nome. **BREAKING**: o console SHALL consumir a representação instance aninhada, com connection, integration e settings; o webhook SHALL ser lido de integration.webhook e os blocos nulos SHALL ser tratados como indisponíveis, sem falhar a listagem nem o detalhe. A busca SHALL usar os campos públicos disponíveis. Referência externa, proprietário e JIDs internos MUST NOT aparecer na representação pública nem ser apagados por campos vazios enviados automaticamente pelo formulário.

#### Scenario: Localizar instância na tabela

- **WHEN** a conta digita parte do nome na busca
- **THEN** a tabela filtra as instâncias carregadas sem nova chamada à API

#### Scenario: Ordenar instâncias

- **WHEN** a conta ordena nome ou estado
- **THEN** as linhas são reordenadas usando name e connection.status

#### Scenario: Paginar instâncias

- **WHEN** a conta navega entre páginas
- **THEN** vê a fatia dos registros carregados mantendo o filtro

#### Scenario: Criar instância

- **WHEN** a conta cria dentro da cota
- **THEN** a instância aparece disconnected com sua key exibida uma vez para cópia

#### Scenario: Remoção com confirmação

- **WHEN** a conta remove uma instância e digita o nome corretamente
- **THEN** ela é removida; com nome divergente, nada acontece

#### Scenario: Cota excedida

- **WHEN** a conta tenta criar acima da cota
- **THEN** o console exibe o erro de cota sem criar nada

#### Scenario: Bloco indisponível no console

- **WHEN** a conta abre uma instância desconectada ou cujo bloco veio nulo
- **THEN** os painéis de integração e configurações mostram os blocos preenchidos e tratam os blocos nulos como indisponíveis, sem erro

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
- **THEN** o overview deriva os mesmos cards da listagem completa obtida em uma chamada e exibe aviso discreto

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

As listas SHALL manter dados, filtros, ordenação, seleção e paginação atuais adotando o `ui` de tabela do template customers (bordas arredondadas, header com fundo) e footer com contagem de selecionados + paginação; o detalhe SHALL adotar header + faixa de stats + navegação por seções via `UNavigationMenu` horizontal no toolbar em desktop (padrão `settings.vue`) e seletor nomeado e legível no mobile, com as mesmas 7 seções (overview, messages, groups, channels, profile, integrations, settings), mantendo os fluxos atuais de pairing, nome, key, webhook, envio e danger; a seção messages SHALL usar o split inbox full-width (lista selecionável + painel de detalhe + composer no rodapé, `USlideover` no mobile) e as demais seções SHALL centralizar forms em `lg:max-w-2xl`.

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

### Requirement: Uso de instâncias fornecido pelo backend

O Manager SHALL usar a contagem de instâncias fornecida pelo backend para representar consumo de quota. Ocultar o dono nos DTOs de instância MUST NOT fazer o consumo aparecer como zero.

#### Scenario: Usuário com instâncias desconectadas

- **WHEN** o Manager apresenta o limite e uso de uma conta com instâncias desconectadas
- **THEN** mostra a contagem retornada pelo backend e mantém a regra de quota existente

### Requirement: Busca de grupo sem identificador

A seção de grupos SHALL recusar a busca quando o campo de JID está vazio ou só tem espaço, e MUST NOT chamar a API nesse caso. O grupo exibido permanece o anterior.

#### Scenario: Busca com campo vazio

- **WHEN** a conta aciona a busca de grupo com o campo de JID vazio
- **THEN** o console não emite `GET` de grupo e mostra o erro de identificador ausente sem trocar o grupo em tela

### Requirement: Consulta única da coleção de instâncias

O Manager SHALL obter a coleção completa com uma única chamada GET /instances sem limit ou cursor para a lista de instâncias, métricas do overview e contagem de uso por conta. A lista SHALL manter busca, filtros, ordenação e paginação visual local, sem botão para carregar páginas do servidor.

#### Scenario: Listagem do console

- **WHEN** a conta carrega a lista de instâncias
- **THEN** todas as instâncias autorizadas ficam disponíveis para busca e paginação visual com uma única consulta da coleção

#### Scenario: Uso por conta

- **WHEN** o administrador carrega o uso de instâncias por conta
- **THEN** a contagem usa todos os itens de uma única consulta GET /instances

### Requirement: Contrato vigente nas mensagens e tipos

O Manager SHALL representar exclusivamente os DTOs atuais e os erros produzidos pelo contrato vigente, sem textos de preservação histórica ou interpretação de envelopes antigos. Falhas atuais MUST continuar visíveis com a mensagem retornada pela API, respeitando os escopos e a privacidade.

#### Scenario: Falha atual exibida

- **WHEN** uma operação atual retorna um erro estruturado
- **THEN** o Manager informa a falha e a mensagem do servidor sem inventar tradução de erro histórico

#### Scenario: Formulário de nome

- **WHEN** o operador abre criação ou edição de instância
- **THEN** as orientações descrevem somente nomes válidos e conflitos do contrato atual

### Requirement: Validação atual de nomes nos formulários

Create, edit and overview forms SHALL explain and validate the same name grammar and reserved names as the API. Every submitted name SHALL satisfy the current grammar; forms MUST NOT offer historical-name exceptions, trimming or silent renaming. Name-conflict errors SHALL be distinguished from external-reference conflicts.

#### Scenario: Invalid new name
- **WHEN** a user enters spaces, accents, invalid punctuation, an overlong value or a reserved name
- **THEN** the form explains the rule and prevents submission

#### Scenario: Name already occupied
- **WHEN** the API returns instance_name_taken
- **THEN** the form reports the instance-name conflict instead of an external-reference conflict

### Requirement: Perfil editado por campos alterados

O manager SHALL permitir alterar ou limpar somente o recado sem enviar nome inalterado. Nome/foto não suportados SHALL continuar apresentando o aviso 501; o manager MUST NOT informar sucesso parcial silencioso quando nome e recado são alterados juntos.

#### Scenario: Apenas recado
- **WHEN** o operador altera ou limpa o recado mantendo o nome
- **THEN** o pedido contém apenas status_text, inclusive vazio explícito, e o recado é aplicado

#### Scenario: Nome realmente alterado
- **WHEN** nome alterado recebe 501, isolado ou junto ao recado
- **THEN** o manager apresenta o aviso sem anunciar atualização concluída

### Requirement: Navegação preservada após rename por alias

Após rename em um detalhe aberto por alias, o manager SHALL recarregar usando identidade válida e SHALL manter URL atualizável com section preservada.

#### Scenario: Ação posterior ao rename
- **WHEN** o detalhe aberto pelo nome antigo é renomeado e depois recarregado ou atualizado
- **THEN** o pedido usa o UUID válido e refresh da URL mantém a instância e a seção

### Requirement: Coordenadas explícitas no envio de localização

O formulário SHALL exigir latitude e longitude presentes antes da conversão numérica. Vazio ou espaços MUST NOT ser interpretados como zero; zero explícito SHALL permanecer válido.

#### Scenario: Coordenada ausente
- **WHEN** pelo menos uma coordenada está vazia ou contém apenas espaços
- **THEN** o manager mostra erro e não chama a API

#### Scenario: Origem explícita
- **WHEN** latitude e longitude são preenchidas explicitamente com zero
- **THEN** o payload contém zero em ambos os campos

### Requirement: Status visível sem legenda

Todo status existente SHALL manter linha identificável e ação de remoção, mesmo sem text/caption. Estado vazio SHALL depender de coleção vazia e não de ausência de legenda.

#### Scenario: Mídia sem legenda
- **WHEN** o status de imagem ou vídeo não possui texto nem legenda
- **THEN** a linha identifica o status e mantém Delete disponível

### Requirement: Revogação independente do cache de key

A disponibilidade de revogação SHALL depender da autorização administrativa e estado da operação, sem exigir reconhecimento da key no cache local. Key recém-rotacionada SHALL poder ser revogada; confirmação e apresentação única SHALL ser preservadas.

#### Scenario: Outro navegador
- **WHEN** admin acessa settings sem cache local ou com storage indisponível
- **THEN** pode confirmar a revogação sem rotacionar antes

#### Scenario: Key recém-rotacionada
- **WHEN** admin revoga a key ainda apresentada uma única vez
- **THEN** sucesso limpa a exibição/cache e falha conserva estado para nova tentativa

### Requirement: Usable form validation

The manager SHALL accept non-negative whole numeric quotas, preserve an explicitly entered zero as unlimited, omit a blank creation quota, and reject invalid quota values before a write request; login validation SHALL describe missing fields without exposing internal validator types.

#### Scenario: Numeric input creation and editing
- **WHEN** an administrator enters 0 or a positive whole quota using the numeric input and submits an otherwise valid account form
- **THEN** the form sends that exact numeric quota and completes successfully

#### Scenario: Empty and invalid quota
- **WHEN** a creation quota is blank or any quota is negative or fractional
- **THEN** a blank creation quota is omitted and an invalid value prevents a write with a readable field error

#### Scenario: Empty sign-in
- **WHEN** a visitor submits an untouched sign-in form
- **THEN** the email and password fields show readable instructions and no login request is sent

### Requirement: Correct accessible control targets

The manager SHALL provide distinct label targets for individual webhook events and accessible names describing icon navigation, file picker and selection controls.

#### Scenario: Webhook event label
- **WHEN** the operator clicks the visible label for a webhook event
- **THEN** only that event's selection changes and its accessible name identifies that event

#### Scenario: Named controls
- **WHEN** assistive technology encounters the instance back action, a photo or message file picker, or the confirmed unnamed selects
- **THEN** it announces the action or field name in the manager's locale

#### Scenario: Selected file preview
- **WHEN** the operator selects an image in a photo or message file picker
- **THEN** the thumbnail has valid alternative text or decorative image semantics, and file name and removal remain available

### Requirement: Legible theme and responsive instance navigation

The manager SHALL keep normal semantic text at a contrast ratio of at least 4.5:1 against its rendered background and keep every instance section label readable and reachable at 320px and wider without page-level horizontal overflow.

#### Scenario: Light semantic colors
- **WHEN** a visitor uses primary actions, active navigation or semantic feedback in light mode
- **THEN** the text meets the contrast requirement including hover states

#### Scenario: Narrow instance navigation
- **WHEN** the instance detail is opened on a narrow viewport
- **THEN** the seven sections remain readable and can be selected, with the current section visible

### Requirement: Available column controls

The manager SHALL show a column-display control only when the current table has optional columns; required instance identity and status columns SHALL remain visible.

#### Scenario: Required instance columns
- **WHEN** the instance table contains only required columns
- **THEN** no empty column-display menu is offered

#### Scenario: Optional account columns
- **WHEN** the account table has optional columns
- **THEN** its display menu allows toggling those columns while retaining row selection and actions

### Requirement: Accessible table row actions

Instance tables SHALL preserve native row semantics so the row's links, selection checkbox and action buttons remain individually accessible; pointer navigation and checkbox keyboard selection SHALL retain the displayed instance identity after filtering and sorting.

#### Scenario: Nested row controls
- **WHEN** an operator traverses an instance table row using assistive technology
- **THEN** its name link, checkbox and actions keep their own accessible roles and names without a containing button role

#### Scenario: Displayed row identity
- **WHEN** an operator filters or sorts the table and opens the displayed instance
- **THEN** navigation targets that displayed instance and checkbox selection remains independent
