## ADDED Requirements

### Requirement: Autorização de sessão vinculada à conta atual

Cada sessão SHALL autorizar usando uma conta ainda existente e sua role atual. Conta removida MUST receber 401 nas rotas privadas; erro ao verificar a conta MUST NOT conceder acesso. API keys SHALL manter sua autenticação independente.

#### Scenario: Conta removida após login
- **WHEN** a conta é removida e seu cookie anterior acessa uma rota privada
- **THEN** a requisição recebe 401 e nenhum efeito é executado

#### Scenario: Role modificada após login
- **WHEN** um cookie emitido como admin acessa gestão de contas após a role atual tornar-se user
- **THEN** a operação recebe 403

#### Scenario: Lookup indisponível
- **WHEN** a conta da sessão não pode ser verificada por erro da persistência
- **THEN** o serviço não autoriza a operação com base na role antiga do cookie

### Requirement: Capacidade limitada da proteção de login

A proteção pública de login SHALL limitar o número de budgets ativos e SHALL preservar o orçamento de IPs já conhecidos quando a capacidade estiver cheia. Entradas expiradas SHALL liberar capacidade; IP novo sem capacidade MUST ser recusado sem criar entrada ilimitada.

#### Scenario: Capacidade cheia
- **WHEN** IPs novos excedem a capacidade antes das entradas existentes expirarem
- **THEN** o número de entradas permanece limitado e budgets ativos não são reiniciados
