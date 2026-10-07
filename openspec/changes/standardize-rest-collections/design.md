## Context

Ver `proposal.md` para motivação e escopo. Os handlers em `internal/httpapi` usam o envelope `data` e DTOs públicos separados dos modelos persistidos. Dez operações ainda serializam `items`; cinco dessas coleções também envolvem cada elemento em uma chave singular. O Manager depende desses formatos em tipos, composables, overview, listas e telas de detalhe.

Os DTOs de instância e mensagem são compartilhados entre coleções e respostas individuais. Hoje `settings` é uma struct por valor com subblocos nullable; a agregação degrada fontes indisponíveis para nil e conserva concorrência limitada. Grupos e canais usam `time.Time` zero quando não há atualização de metadados conhecida. Toda instância possui estado de webhook persistido, sem marcador de configuração explícita; Chatwoot distingue configuração persistida de ausência por existência do registro.

## Goals / Non-Goals

**Goals:** expressar presença corretamente nos DTOs públicos, conservar valores válidos e arrays obrigatórios vazios, garantir representações equivalentes entre coleção e detalhe e atualizar os consumidores junto do contrato; migrar todas as rotas para Chi e organizar o transporte em pacotes por recurso com interfaces menores.

**Non-Goals:** mudar modelos de domínio/persistência, payloads de escrita, eventos, algoritmos de paginação/ordenação ou comportamento de replay; acrescentar dependências além de Chi, migrações, aliases ou serialização JSON customizada; adotar outros frameworks, OpenAPI 3, geração de código ou alterações de CI. Configurações standalone de Chatwoot conservam seu estado padrão desativado quando não há registro. A migração é da camada HTTP completa; a arquitetura dos serviços de negócio e workers permanece fora do escopo.

## Decisions

### 1. Coleções diretas nomeadas pelo recurso

| Operação | Coleção dentro de data | Elemento atual reutilizado |
|---|---|---|
| GET /instances | instances | instanceResponse |
| GET /users | users | userResponse |
| GET /instances/{id}/groups | groups | groupResponse |
| GET /instances/{instance}/messages | messages | messageResponse |
| GET /instances/{id}/newsletters | channels | newsletterResponse |
| GET /instances/{id}/newsletters/{channel}/messages | messages | newsletterMessageResponse |
| GET /instances/{id}/newsletters/{channel}/updates | messages | newsletterMessageResponse |
| GET /instances/{id}/status/updates | statuses | ownStatusResponse |
| POST /instances/{id}/contacts/check | contacts | contactCheckItem |
| GET /instances/{id}/blocklist | blocked_jids | string |

`channels` descreve metadados de canais com título, seguidores e identificador; `statuses` descreve publicações próprias do WhatsApp, sem relação com o estado de conexão. A consulta de atualizações de canal retorna o mesmo DTO de mensagem da consulta de mensagens, portanto usa `messages`, mantendo a rota `/updates`. `contacts` conserva a semântica de checagem em lote e `blocked_jids` conserva elementos string.

Leituras individuais preservam `data.instance`, `data.user`, `data.group`, `data.message` e `data.channel`. Os handlers e o agregador de listagem usam os mesmos mapeadores de recurso das leituras individuais. Diferenças contextuais vigentes, como convite exclusivo da criação e resposta resumida de aceite, permanecem.

Alternativas: conservar `items`, pluralizar apenas a rota ou conservar wrappers por elemento; rejeitadas porque tornam o recurso implícito ou adicionam uma camada redundante. Não adicionar campos ou aliases para permitir leitura dos dois formatos.

### 2. Matriz explícita de serialização

| Situação | JSON | Modelagem Go |
|---|---|---|
| Campo opcional ausente | omitido | tipo apropriado com omitempty |
| false obrigatório | false | bool sem omitempty |
| 0 obrigatório | 0 | tipo numérico sem omitempty |
| "0" obrigatório | "0" | string sem omitempty |
| Slice obrigatório vazio | [] | []T não nil sem omitempty |
| Slice opcional ausente | omitido | *[]T nil quando ausência e vazio forem distintos |
| Objeto opcional ausente | omitido | *DTO nil com omitempty |
| Data opcional desconhecida | omitida | *time.Time nil com omitempty |
| next_cursor ausente | omitido | string vazia com omitempty |

Não aplicar `omitempty` indiscriminadamente. Campos obrigatórios permanecem sem a opção; campos opcionais booleanos ou numéricos exigem ponteiros quando zero é um valor configurado válido. Uma string opcional pode usar `omitempty` diretamente somente quando string vazia significa ausência; nunca inferir ausência de strings com conteúdo, inclusive `"0"`. Para um slice opcional que distingue ausência de vazio configurado, nil no ponteiro omite o campo e ponteiro para slice vazio não nil produz `[]`; esta change não introduz novos campos de coleção opcionais.

DTOs de coleção e arrays obrigatórios aninhados não usam `omitempty`. Construções por mapeamento inicializam o destino com `make([]DTO, 0, len(source))`; atribuições diretas normalizam nil para slice vazio antes de serializar. Conservar essa regra em eventos do webhook, participantes, JIDs de privacidade e `ignored_jids` do Chatwoot. Aplicar no transporte, sem modificar os retornos nil dos serviços de domínio.

Alternativas: omitir coleções vazias, migrar encoder JSON, gerar MarshalJSON ou utilizar omitzero como substituto indiscriminado; rejeitadas para manter o contrato explícito e usar o encoding/json atual.

### 3. Objetos opcionais e valores presentes

```go
type instanceResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Connection  connectionResponse  `json:"connection"`
	Integration integrationResponse `json:"integration"`
	Settings    *settingsResponse   `json:"settings,omitempty"`
}

type integrationResponse struct {
	Webhook        webhookResponse         `json:"webhook"`
	ChatwootConfig *chatwootConfigResponse `json:"chatwoot_config,omitempty"`
}

type webhookResponse struct {
	Enabled bool     `json:"enabled"`
	URL     *string  `json:"url,omitempty"`
	Events  []string `json:"events"`
}

type settingsResponse struct {
	DefaultDisappearing *string                `json:"default_disappearing,omitempty"`
	Profile             *profileResponse       `json:"profile,omitempty"`
	Privacy             *privacyResponse       `json:"privacy,omitempty"`
	StatusPrivacy       *statusPrivacyResponse `json:"status_privacy,omitempty"`
}

type instanceListResponse struct {
	Instances []instanceResponse `json:"instances"`
}

type messageListResponse struct {
	Messages   []messageResponse `json:"messages"`
	NextCursor string            `json:"next_cursor,omitempty"`
}
```

Esses trechos reutilizam os demais DTOs existentes. A ordem das declarações facilita a leitura; ordem textual do JSON não faz parte do contrato nem dos testes. Não ordenar propriedades via encoder customizado.

Construir configurações em variável local, preencher o timer persistido e os blocos vivos disponíveis e atribuir `Settings` somente se pelo menos um dos quatro campos tiver valor; isso vale tanto no retorno de instância desconectada quanto após o término das leituras concorrentes. Um timer com ponteiro para `"0"` é suficiente para manter `settings` presente. A ausência de um bloco não remove os demais. Preservar o limite atual de concorrência da agregação e a ordem dos recursos na listagem.

`integration` e seu webhook são structs por valor obrigatórias. Desativar webhook não remove sua URL ou eventos configurados. Chatwoot é ponteiro opcional na agregação; registro persistido desativado produz objeto presente, ausência ou falha de leitura produz nil. Sua cópia aninhada continua sem `instance_id` e token. A leitura standalone conserva o objeto padrão desativado sem registro.

Alternativas: manter settings como struct por valor com omitempty ou esconder webhooks sem URL; rejeitadas porque struct vazia não representa ausência e URL não indica o histórico de configuração do webhook. Nenhum marcador novo será persistido.

### 4. Catálogo dos opcionais compartilhados

| DTO | Ajuste |
|---|---|
| lastErrorResponse | occurred_at como *time.Time com omitempty; code/message obrigatórios |
| connectionResponse | last_error e last_connected_at omitidos quando nil; manter status obrigatório e opcionais QR atuais |
| settingsResponse | quatro subblocos com ponteiros e omitempty |
| instanceResponse | settings como ponteiro com omitempty; identidade, timestamps, connection e integration obrigatórios |
| messageResponse | wa_id, media_id, last_error, next_attempt_at, delivered_at e read_at com omitempty; retry_count obrigatório |
| acceptedMessageResponse | media_id omitido quando nil; conservar resposta resumida obrigatória |
| groupResponse / newsletterResponse | updated_at como *time.Time com omitempty quando origem zero significa atualização desconhecida |
| chatwootConfigResponse | url, account_id, organization e logo como strings opcionais com omitempty; flags, import_days, ignored_jids e defaults efetivos preservados |
| profileResponse | photo_url omitido quando vazio; name e status_text conservam presença, inclusive conteúdo vazio válido |
| DTOs paginados | next_cursor string com omitempty, junto da coleção |

Não transformar datas obrigatórias de instâncias ou mensagens em opcionais. Não substituir datas obrigatórias ou desconhecidas por `time.Now()`. Normalizar para nil somente a sentinela de ausência reconhecida na origem: `IsZero()` para atualização de metadados de grupos/canais e ponteiros nil para os marcos opcionais já modelados.

```go
type lastErrorResponse struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	OccurredAt *time.Time `json:"occurred_at,omitempty"`
}

type connectionResponse struct {
	Status          string             `json:"status"`
	LastError       *lastErrorResponse `json:"last_error,omitempty"`
	LastConnectedAt *time.Time         `json:"last_connected_at,omitempty"`
	QRCode          string             `json:"qr_code,omitempty"`
	QRExpiresAt     *time.Time         `json:"qr_expires_at,omitempty"`
}
```

### 5. Garantia de arrays e consumo no Manager

```go
func newInstanceListResponse(instances []instanceResponse) instanceListResponse {
	if instances == nil {
		instances = []instanceResponse{}
	}
	return instanceListResponse{Instances: instances}
}

func newWebhookResponse(hook model.InstanceWebhook) webhookResponse {
	events := hook.Events
	if events == nil {
		events = []string{}
	}
	return webhookResponse{
		Enabled: hook.IsEnabled,
		URL:     hook.URL,
		Events:  events,
	}
}
```

Aplicar a mesma normalização às demais coleções e construtores relevantes; não centralizar uma limpeza genérica que retire valores válidos. Remover wrappers somente dos DTOs de coleção, mantendo envelopes singulares utilizados pelos endpoints individuais.

No Manager, interfaces de coleção passam a conter arrays diretos, obrigatórios; opcionais passam a propriedades `?` sem obrigatoriedade de `null`. `settings`, foto, erros e datas ausentes são tratados na UI com optional chaining, fallbacks de indisponibilidade e formatação que aceite undefined. Valores zero devem ser testados por presença, evitando fallback baseado apenas em truthiness. Atualizar overview, contas, mensagens, grupos, canais, statuses e fixtures. `next_cursor?: string` determina a existência da próxima página.

Não converter `undefined` em valores de escrita que apaguem configurações; inputs e formatos de requisição permanecem atuais. Rótulos e traduções seguem os padrões existentes do Manager.

### 6. Testes estruturais e documentação

Atualizar a matriz HTTP de respostas e os testes de DTOs/Swagger para os nomes finais, obrigatoriedade de arrays e opcionais. Exercitar retornos nil das fontes, zeros válidos, settings parcial, falta de timestamps, equivalência lista/detalhe e privacidade. Decodificar JSON e verificar presença, tipo e valor; usar mapas ou RawMessage para distinguir omitido, null e []. Não comparar ordem textual ou criar snapshots dependentes dela. Assertions de replay byte-a-byte continuam válidas porque verificam fidelidade do corpo armazenado, sem inferir ordem contratual das propriedades.

Atualizar README, comentários de DTOs, orientação de contrato em AGENTS.md, annotations e Swagger gerado. Os schemas devem refletir arrays obrigatórios não nullable e opcionais não obrigatórios; não tornar obrigatório um campo somente porque possui valor zero no exemplo. Reutilizar as dependências e o gerador Swag atuais.

### 7. Migração integral para Chi e pacotes por recurso

Chi v5.3.2 é o único roteador do transporte, mantendo `http.Server` e handlers padrão. Manter a release fixada; Chi é a única nova dependência direta de produção. Não adotar outros frameworks ou geradores, wrappers de ServeMux, normalização histórica de padrões ou aliases.

O pacote raiz `internal/httpapi` monta o servidor e os routers. Pacotes de recurso contêm handlers reais, DTOs específicos, interfaces mínimas dos serviços consumidos e registro de rotas: `instances`, `messages`, `groups`, `contacts`, `channels`, `chats`, `statuses`, `profile`, `users`, `authsession`, `media` e `chatwoot`. Distribuir parity_* por essas responsabilidades. Nenhum pacote novo apenas delega a handlers mantidos na raiz.

`core` contém envelopes, decode/limites, middleware, RBAC, resolução de instância, idempotência e erros comuns. `representation` contém DTOs e mapeadores públicos compartilhados, incluindo os blocos usados na agregação. A direção das dependências é raiz → recursos → core/representation/domínio. Recursos não importam outros handlers nem o pacote raiz; core/representation não importam handlers. Não mover serviços de negócio, persistência ou workers e não criar um framework interno genérico.

Contrato final de roteamento: raiz desconhecida responde 404 JSON; namespaces privados `/instances`, `/users` e `/media` autenticam antes dos handlers e dos fallbacks; métodos não permitidos respondem 405 JSON com Allow derivado das rotas finais. Usar GetHead para atender HEAD em operações GET, conservando o método original e incluindo HEAD em Allow. Health/readiness, Swagger, Manager, authsession e webhook Chatwoot possuem registro público explícito. HTML, mídia binária, reconhecimento de webhook e 204 conservam seus formatos específicos. Não reproduzir peculiaridades do roteador substituído nem introduzir aliases de barra final.

Middleware no escopo de instância resolve UUID ou nome exato, autoriza o alvo e guarda a instância e UUID canônico no contexto. Handlers e idempotência usam o contexto; não reescrever o padrão Chi e não depender da substituição de PathValue em routers aninhados. Chaves de instância resolvem somente seu próprio alvo e não consultam nomes estrangeiros. Resolver do webhook público usa o mesmo alvo canônico sem exigir RBAC de chamadas autenticadas.

Nas rotas de mensagens, o parâmetro ancestral é `instance` e o identificador da mensagem é `id`: `GET /instances/{instance}/messages/{id}`. O primeiro aceita UUID ou nome exato; o segundo é o UUID interno retornado em `data.message.id`. O resolver recebe explicitamente o nome do parâmetro da instância conforme o registro da rota. Não reutilizar `id` para dois valores distintos na mesma rota nem criar aliases de parâmetros. Essa nomenclatura é usada no Go, no inventário e no Swagger; a URL concreta continua, por exemplo, `/instances/abc/messages/xyz`. As demais famílias preservam os nomes de seus parâmetros.

A identificação idempotente usa diretamente o padrão nativo Chi e o conteúdo da requisição. Conservar a rejeição de chave associada a fingerprint diferente e a reprodução literal de uma resposta da mesma operação e fingerprint. Não converter nem apagar registros do banco; conflitos expiram segundo a política de 24h do mecanismo. Não implementar identificação alternativa para transportar formatos anteriores. A proteção permanece explicitamente vinculada às operações selecionadas no inventário, sem ampliar silenciosamente comandos que não a utilizam.

Testes de integração verificam New e a API HTTP pública; testes de helpers locais acompanham core/representation/recursos, com suporte de testes independente. Não manter aliases ou bridges de teste para sustentar a API privada de arquivos movidos. A verificação Swagger usa o inventário real de Chi, por chi.Walk, e verifica todas as operações documentadas, excluindo mounts e métodos auxiliares quando aplicável. Preservar Swaggo/Swagger 2.0 e regenerar os schemas a partir dos tipos finais.

Alternativas: simples agrupadores delegando a handlers na raiz, compatibilidade com padrões substituídos, divisão dos serviços de negócio e uso de OpenAPI/codegen ou Huma; a arquitetura aprovada usa Chi nativo e migração real dos recursos.

## Risks / Trade-offs

- [Risk] Clientes e Manager continuam esperando items/wrappers -> Mitigation: entregar backend, Manager, exemplos e Swagger juntos, marcando BREAKING.
- [Risk] omitempty remove false, 0 ou [] -> Mitigation: classificar campo a campo, não usar a opção em obrigatórios e testar os valores pela presença e pelo tipo.
- [Risk] Sources retornam slices nil ou settings all-nil é emitido como {} -> Mitigation: normalizar em construtores/mapeamento e decidir explicitamente a presença do ponteiro de settings depois da agregação.
- [Risk] Datas desconhecidas são publicadas como ano 0001 -> Mitigation: usar *time.Time e converter sentinelas reconhecidas para nil, preservando timestamps obrigatórios.
- [Risk] Opcionais ausentes causam erro no Manager ou removem valores configurados -> Mitigation: propriedades opcionais nos tipos, tratamento de undefined na UI e testes de campos ausentes e zeros.
- [Risk] DTO compartilhado muda detalhe ou aceite de forma diferente da listagem -> Mitigation: mapeadores compartilhados, testes de equivalência por mesmo estado/contexto e preservação dos formatos resumidos e campos exclusivos de criação.
- [Risk] Alteração do contrato modifica eventos, segredos ou replays -> Mitigation: manter DTOs de transporte separados, exclusão de campos internos e testes do contrato final de privacidade, eventos e replay sem conversão.

- [Risk] Migração deixa rotas ou testes dependentes da estrutura substituída -> Mitigation: inventário completo, testes do contrato Chi final e remoção de wrappers, bridges e normalizações de padrões.
- [Risk] Separação introduz ciclos ou infraestrutura compartilhada ampla -> Mitigation: dependências unidirecionais, interfaces locais menores e handlers efetivamente dentro dos recursos.
- [Risk] Teste Swagger analisa somente server.go -> Mitigation: descobrir todas as operações registradas por recurso e verificar cobertura completa antes da entrega.

## Migration Plan

1. No apply, criar plan.md com micro-passos TDD referenciados pelos IDs de tasks.md e usar worktree em .worktrees/standardize-rest-collections; conferir a preparação do embed do Manager e baseline antes dos testes Go.
2. Atualizar testes antes do código, confirmar falhas de contrato esperadas, migrar roteamento para Chi e separar handlers/DTOs por recurso; implementar mapeadores e atualizar consumidores após estabilizar o contrato.
3. Atualizar documentação e regenerar Swagger; validar o change com openspec validate standardize-rest-collections --strict.
4. Executar gofmt, testes focados e gerais, go vet, golangci-lint v2.13.2, go build, testes/lint/typecheck/build do Manager e freshness do Swagger.
5. Integração Postgres somente com WZAP_TEST_DATABASE_URL definido, DB acessível com nome _test e schema isolado; NATS somente com WZAP_TEST_NATS_URL definido e serviço acessível; não iniciar segunda réplica.
6. Produzir verify.md e retrospective.md após apply; publicação conjunta dos componentes deve indicar BREAKING e os novos caminhos de leitura.
7. Rollback restaura os componentes da versão anterior em conjunto, sem migração de banco ou conversão de respostas idempotentes armazenadas; arquivar somente depois da implementação e verificação.
