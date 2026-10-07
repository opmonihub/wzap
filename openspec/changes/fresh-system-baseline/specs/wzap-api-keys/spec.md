## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Header apikey

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Credenciais de máquina do contrato vigente".

**Migration:** usar somente a instalação nova e o requisito "Credenciais de máquina do contrato vigente", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Credenciais de máquina do contrato vigente" sem depender de cenários históricos deste requisito

### Requirement: Rotação e revogação

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Rotação e revogação de keys no sistema atual".

**Migration:** usar somente a instalação nova e o requisito "Rotação e revogação de keys no sistema atual", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Rotação e revogação de keys no sistema atual" sem depender de cenários históricos deste requisito
