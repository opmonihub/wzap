# wzap-api-keys Specification

## Purpose

Credenciais de máquina para integrações e revenda: uma key global de operador
e uma key por instância, resolvidas junto com a sessão de usuário no mesmo
controle de acesso.

## Requirements

### Requirement: Key global

A key global SHALL autorizar todas as operações, incluindo gerência de contas,
cotas, todas as instâncias e ciclo de vida de keys. Ela SHALL ser configurada
por ambiente e MUST NOT trafegar em respostas da API.

#### Scenario: Operação global

- **WHEN** o cliente usa a key global em qualquer rota autenticada
- **THEN** a operação é autorizada, respeitadas as demais regras de negócio

### Requirement: Key por instância

Cada instância SHALL ter no máximo uma key ativa, que autoriza controle total
da própria instância — pareamento, QR, estado, envio, mensagens, verificação
de número, desconexão e remoção — e MUST NOT autorizar nenhuma operação fora
dela, incluindo gerenciar a própria key. Uso fora do escopo MUST responder
`403`; rotas de coleção geral (ex.: listar todas as instâncias) MUST responder
`403` para keys de instância.

#### Scenario: Cliente opera a própria instância

- **WHEN** o cliente usa a instance key nas rotas daquela instância
- **THEN** todas as operações são autorizadas

#### Scenario: Key cruza instâncias

- **WHEN** o cliente usa a key da instância A numa rota da instância B
- **THEN** a resposta é `403`

#### Scenario: Key gerencia a si mesma

- **WHEN** o cliente usa a instance key para rotacionar ou revogar a própria key
- **THEN** a resposta é `403`

### Requirement: Emissão única na criação

A criação de instância SHALL emitir sua key e devolvê-la em claro uma única
vez na resposta; o serviço MUST armazenar somente um derivado não reversível.
A key em claro MUST NOT ser recuperável depois por nenhuma rota.

#### Scenario: Key exibida uma vez

- **WHEN** a instância é criada com sucesso
- **THEN** a resposta traz a key em claro e consultas posteriores não a revelam

### Requirement: Independência da conta dona

A validade da instance key MUST NOT depender da existência ou do estado da
conta dona; somente rotação e revogação afetam a key.

#### Scenario: Key sobrevive ao dono

- **WHEN** a conta dona é removida após suas instâncias serem removidas ou a
  key é usada por uma integração enquanto o dono está inativo
- **THEN** a key segue válida até ser rotacionada ou revogada

### Requirement: Resolução dual de credencial

Cada rota autenticada SHALL aceitar a sessão de usuário OU a apikey, com
escopos equivalentes: sessão `admin` equivale à key global; sessão `user`
restrita às instâncias próprias equivale, dentro de cada uma, à instance key.

#### Scenario: Mesma operação pelos dois planos

- **WHEN** a mesma operação é feita com sessão de usuário dono e com a instance
  key
- **THEN** ambas são autorizadas com o mesmo efeito observável

### Requirement: Credenciais de máquina do contrato vigente

Toda requisição autenticada de máquina SHALL enviar a credencial no header
`apikey:`; outros esquemas de credencial de máquina MUST NOT ser aceitos. Credencial ausente ou inválida MUST responder `401` sem revelar
detalhes.

#### Scenario: Requisição com key válida

- **WHEN** o cliente envia uma apikey válida no header `apikey:`
- **THEN** a requisição é processada conforme o escopo da key

#### Scenario: Key ausente ou inválida

- **WHEN** o cliente omite o header ou envia key desconhecida
- **THEN** a resposta é `401` sem detalhes que permitam inferir uma key válida

#### Scenario: Esquema de máquina não suportado

- **WHEN** o cliente envia `Authorization: Bearer` com qualquer valor
- **THEN** a resposta é `401`, como se nenhuma credencial tivesse sido enviada

### Requirement: Rotação e revogação de keys no sistema atual

Somente a key global ou sessão `admin` SHALL rotacionar (criar/substituir) ou
revogar a key de uma instância. Rotacionar MUST invalidar a key anterior
imediatamente e devolver a nova em claro uma única vez. Revogar MUST deixar a
instância acessível somente pela key global ou sessão `admin`, até nova
rotação. Após uma revogação, nova rotação SHALL emitir uma nova key para a mesma instância.

#### Scenario: Rotação

- **WHEN** o operador rotaciona a key de uma instância
- **THEN** a key antiga passa a responder `401` e a nova é devolvida uma vez

#### Scenario: Revogação

- **WHEN** o operador revoga a key de uma instância
- **THEN** a key antiga responde `401` e a instância segue operável pela global

#### Scenario: Nova emissão após revogação

- **WHEN** global ou admin rotaciona a key de uma instância cuja key foi revogada
- **THEN** uma nova key é devolvida uma única vez e somente ela autentica novas chamadas de máquina da instância
