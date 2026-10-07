## ADDED Requirements

### Requirement: Idempotência do contrato vigente

Requisições de envio SHALL aceitar Idempotency-Key. Mesmo conteúdo SHALL recuperar o resultado anterior marcado como replay; chave com conteúdo diferente MUST responder 422; concorrência com a mesma chave MUST responder 409. Sem chave o comportamento é normal. O replay SHALL devolver o status HTTP e o corpo armazenados no contrato atual sem conversão, reenvio ou criação de nova mensagem.

#### Scenario: Replay idempotente

- **WHEN** o cliente repete o envio com a mesma chave e conteúdo
- **THEN** recupera o mesmo UUID em data.message com indicador de replay sem nova mensagem

#### Scenario: Conflito de conteúdo

- **WHEN** o cliente repete a chave com conteúdo diferente
- **THEN** recebe 422 e a mensagem original permanece intacta

#### Scenario: Chave em processamento

- **WHEN** dois envios concorrentes disputam a mesma chave e conteúdo
- **THEN** a requisição concorrente recebe 409 e apenas a operação que adquiriu a chave pode produzir o envio

## REMOVED Requirements

### Requirement: Idempotência de envio

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Idempotência do contrato vigente".

**Migration:** usar somente a instalação nova e o requisito "Idempotência do contrato vigente", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Idempotência do contrato vigente" sem depender de cenários históricos deste requisito
