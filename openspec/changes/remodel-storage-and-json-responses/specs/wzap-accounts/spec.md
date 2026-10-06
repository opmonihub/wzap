## ADDED Requirements

### Requirement: Projeção de limite e uso de instâncias

A representação autorizada de usuário SHALL informar instance_limit e instances_used calculado pelo backend a partir de todas as instâncias possuídas. Esse uso MUST incluir instâncias desconectadas/em erro e ser zero quando nenhuma existir. As regras existentes de limite zero ilimitado, bypass admin e ownership SHALL ser preservadas.

#### Scenario: Instâncias em estados distintos

- **WHEN** a conta possui instâncias conectadas, desconectadas e em erro
- **THEN** a projeção contabiliza todas sem depender de owner_user_id nas respostas públicas de instância

#### Scenario: Conta sem instâncias

- **WHEN** uma conta não possui instâncias
- **THEN** sua projeção informa instances_used zero
