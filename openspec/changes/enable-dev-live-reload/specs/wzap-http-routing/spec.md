## ADDED Requirements

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
