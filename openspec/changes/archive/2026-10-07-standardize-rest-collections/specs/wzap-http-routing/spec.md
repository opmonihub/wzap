## ADDED Requirements

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
