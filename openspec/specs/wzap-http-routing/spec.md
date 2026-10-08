# wzap-http-routing Specification

## Purpose

Estabelecer o roteamento HTTP completo do serviço — métodos, caminhos, autenticação, autorização e idempotência — sem aliases de compatibilidade, respondendo cada operação no contrato JSON final.

## Requirements

### Requirement: Roteamento HTTP completo

O serviço SHALL atender todas as operações previstas no inventário desta change por método e caminho usando o contrato final. Rotas SHALL aplicar autenticação, autorização e limites próprios da operação; somente as mudanças JSON explicitamente previstas nesta change SHALL alterar representações de recursos. Não SHALL existir suporte simultâneo a formatos substituídos ou aliases de compatibilidade.

#### Scenario: Operação autenticada

- **WHEN** um cliente chama uma operação privada com método, caminho, credenciais e payload válidos
- **THEN** a operação executa no escopo autorizado e responde usando o contrato JSON final

#### Scenario: HEAD em GET

- **WHEN** um cliente usa HEAD em caminho atendido por GET
- **THEN** o serviço atende a operação com o status aplicável sem corpo HTTP

#### Scenario: Caminho desconhecido na raiz

- **WHEN** uma requisição consulta caminho não registrado fora dos namespaces privados
- **THEN** recebe 404 no envelope de erro JSON

#### Scenario: Namespace privado sem credencial

- **WHEN** uma requisição sem credencial válida consulta um namespace privado, incluindo seus caminhos desconhecidos
- **THEN** recebe 401 antes de acessar handlers ou fallback do namespace

#### Scenario: Método não permitido

- **WHEN** um cliente autorizado usa método não permitido em um caminho registrado
- **THEN** recebe 405 JSON e Allow correspondente às rotas finais, incluindo HEAD quando GET estiver disponível

#### Scenario: Formatos específicos

- **WHEN** um cliente consulta health, readiness, Swagger, Manager, mídia ou recebe 204 ou reconhecimento do webhook Chatwoot
- **THEN** recebe o formato específico documentado para a operação

#### Scenario: Identificadores independentes no detalhe de mensagem

- **WHEN** um cliente consulta `GET /instances/{instance}/messages/{id}` com a referência da instância e o UUID interno de uma mensagem desse escopo
- **THEN** a operação resolve e autoriza a instância separadamente do identificador da mensagem e documenta os parâmetros `instance` e `id` sem duplicação no Swagger


### Requirement: Alvo autorizado e idempotência

O serviço SHALL resolver a referência UUID ou nome exato da instância e aplicar autorização antes de adquirir ou reproduzir uma chave idempotente. A mesma chave e fingerprint SHALL reproduzir literalmente a resposta armazenada; fingerprint diferente SHALL produzir conflito sem executar novamente a operação. A migração MUST NOT converter nem apagar registros do banco ou introduzir identificação alternativa de requisições.

#### Scenario: Nome da instância

- **WHEN** um cliente autorizado utiliza o nome exato atual da instância
- **THEN** a operação usa o mesmo alvo canônico identificado pelo UUID

#### Scenario: Escopo de chave de instância

- **WHEN** uma chave de instância tenta resolver um nome de outra instância
- **THEN** o serviço rejeita o acesso sem consultar a existência do nome estrangeiro

#### Scenario: Replay da mesma operação

- **WHEN** uma requisição autorizada repete a operação com a mesma chave e fingerprint
- **THEN** recebe o status e o corpo armazenados literalmente, com o header de replay

#### Scenario: Fingerprint diferente

- **WHEN** uma requisição reutiliza uma chave associada a outro fingerprint
- **THEN** recebe o erro de conflito de conteúdo sem executar novamente a operação

#### Scenario: Replay sem autorização

- **WHEN** uma requisição possui chave conhecida mas não possui autorização para a instância
- **THEN** é rejeitada antes de consultar ou reproduzir a resposta armazenada

### Requirement: Encaminhamento do manager em desenvolvimento

Com o encaminhamento dev habilitado, o serviço SHALL redirecionar `/manager` para `/manager/` com `301`, preservando a query string, e SHALL encaminhar requisições sob `/manager/` ao frontend dev, preservando o caminho completo, a query string e o Host público. O encaminhamento SHALL abranger páginas internas, assets e WebSocket/HMR; as rotas da API, probes e Swagger MUST continuar atendidas pelo backend na raiz.

#### Scenario: Entrada canônica com query

- **WHEN** o navegador solicita `/manager?next=instances` em modo dev
- **THEN** recebe `301` com destino `/manager/?next=instances`

#### Scenario: Página interna e asset com query

- **WHEN** o navegador solicita uma página interna ou asset sob `/manager/` com query string
- **THEN** o frontend dev recebe o caminho completo, a query string e o Host da entrada pública

#### Scenario: Upgrade WebSocket

- **WHEN** o navegador solicita um upgrade WebSocket no namespace do manager com o frontend dev disponível
- **THEN** a entrada pública encaminha o upgrade e permite comunicação nos dois sentidos após a resposta `101`

#### Scenario: API permanece no backend

- **WHEN** um cliente chama `/auth`, `/instances`, `/users`, `/media`, `/healthz`, `/readyz` ou `/swagger/` em modo dev
- **THEN** a requisição segue o roteamento e as regras existentes do backend sem ser encaminhada ao frontend

### Requirement: Indisponibilidade explícita do frontend dev

Com o encaminhamento dev habilitado, uma falha de conexão com o frontend SHALL produzir `503` para a requisição de painel ou asset. O serviço MUST NOT substituir essa falha por um bundle estático antigo e MUST NOT expor detalhes internos ou segredos na resposta de erro.

#### Scenario: Frontend parado

- **WHEN** o frontend dev não está acessível e o navegador solicita `/manager/`, uma página interna ou um asset
- **THEN** recebe `503` sem fallback estático

#### Scenario: Frontend recuperado

- **WHEN** o frontend dev volta a responder e o navegador repete a requisição
- **THEN** o serviço volta a encaminhar a requisição sem exigir reinício manual do backend
