## ADDED Requirements

### Requirement: Erros estruturados do contrato vigente

**BREAKING:** erros de conexão e envio SHALL ser objetos com code, message e occurred_at, ou null quando não houver erro. Falhas produzidas pelo sistema atual MUST registrar código específico, mensagem e instante de ocorrência, sem inferência de código a partir de texto histórico. Dados internos e segredos MUST NOT aparecer no erro público.

#### Scenario: Falha atual

- **WHEN** uma operação de conexão ou envio registra uma falha
- **THEN** a resposta apresenta o código, mensagem e instante registrados para aquela falha

#### Scenario: Sem falha registrada

- **WHEN** uma instância ou mensagem não tem erro registrado
- **THEN** last_error é null

### Requirement: Replay do resultado atual armazenado

**BREAKING:** a resposta idempotente SHALL reproduzir o status HTTP e o corpo armazenados pelo contrato atual, sem conversão de formato, leitura substitutiva do recurso ou repetição da operação. A autorização atual MUST ser verificada antes do replay; identificadores, headers de correlação e indicador de replay SHALL preservar sua semântica. Corrupção de um resultado armazenado MUST produzir erro interno sem reexecutar a operação.

#### Scenario: Resultado armazenado

- **WHEN** um cliente atualmente autorizado repete a chave e conteúdo de uma operação concluída
- **THEN** recebe o mesmo status HTTP e corpo armazenado com indicador de replay, sem segundo efeito

#### Scenario: Estado posterior do recurso

- **WHEN** o recurso muda depois de a resposta original ser armazenada e ocorre replay autorizado
- **THEN** a resposta mantém o resultado original sem reconstruí-lo a partir do estado novo

#### Scenario: Autorização atual insuficiente

- **WHEN** um cliente sem escopo atual tenta obter uma resposta armazenada
- **THEN** recebe 403 sem exposição do resultado

## REMOVED Requirements

### Requirement: Erros públicos estruturados

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Erros estruturados do contrato vigente".

**Migration:** usar somente a instalação nova e o requisito "Erros estruturados do contrato vigente", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Erros estruturados do contrato vigente" sem depender de cenários históricos deste requisito

### Requirement: Replay sem repetir efeitos

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Replay do resultado atual armazenado".

**Migration:** usar somente a instalação nova e o requisito "Replay do resultado atual armazenado", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Replay do resultado atual armazenado" sem depender de cenários históricos deste requisito
