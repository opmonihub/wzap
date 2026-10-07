## MODIFIED Requirements

### Requirement: Autenticação de serviço

Todos os endpoints da API, exceto saúde, prontidão, console e documentação, SHALL exigir credencial válida: sessão de usuário OU header `apikey:`. Credencial ausente ou inválida MUST responder `401` sem revelar detalhes; credencial válida sem escopo para a operação MUST responder `403`.

#### Scenario: Requisição autenticada

- **WHEN** o cliente envia sessão válida ou apikey válida com escopo
- **THEN** a requisição é processada

#### Scenario: Token inválido

- **WHEN** o cliente envia credencial ausente ou inválida (incluindo esquemas de máquina não suportados)
- **THEN** a resposta é `401` sem detalhes que permitam inferir credencial válida

#### Scenario: Sem escopo

- **WHEN** o cliente envia credencial válida para operação fora do seu escopo
- **THEN** a resposta é `403`

### Requirement: Migrações

**BREAKING:** o serviço SHALL instalar diretamente o schema vigente sobre banco vazio quando a migração automática estiver habilitada e MUST refletir migrações pendentes na prontidão. Futuras alterações SHALL partir dessa base, sem cadeia de upgrade ou tratamento de dados de versões anteriores.

#### Scenario: Migrações pendentes

- **WHEN** existem migrações não aplicadas e a aplicação automática está habilitada
- **THEN** elas são aplicadas antes de o serviço aceitar tráfego

## ADDED Requirements

### Requirement: Comandos do contrato vigente

O binário SHALL expor serve como padrão, migrate para aplicar o schema e healthcheck para consultar prontidão. Outros subcomandos MUST produzir erro de comando desconhecido, sem acionamento de transferência ou conversão de mídia.

#### Scenario: Comandos atuais

- **WHEN** o operador executa serve, migrate ou healthcheck
- **THEN** o binário executa a operação documentada para o comando

#### Scenario: Comando desconhecido

- **WHEN** o operador informa um subcomando não disponível
- **THEN** o binário termina com erro sem executar outra operação

### Requirement: Reinicialização local sem preservação de dados

O reset desta change SHALL ocorrer somente após os gates de implementação e SHALL descartar banco, sessões, mensagens, mídia e eventos do ambiente local, sem backup ou restauração. A remoção MUST se limitar aos quatro volumes identificados do projeto. A instalação reconstruída SHALL operar como uma única réplica, usando S3/MinIO por padrão no Compose e exigindo novo cadastro e pareamento.

#### Scenario: Reset local aprovado

- **WHEN** os gates da implementação passam e o reset local autorizado é executado
- **THEN** somente os quatro volumes previstos são descartados sem backup e o serviço inicia com dados vazios

#### Scenario: Novo uso

- **WHEN** o operador acessa o ambiente reconstruído com seed configurado
- **THEN** pode autenticar-se com o admin inicial e criar uma instância que precisa de novo pareamento

### Requirement: Rotas do contrato vigente na raiz

As rotas da API SHALL viver na raiz, sem prefixo de versão (`/instances`, `/instances/{id}/messages/text`, `/media/{id}`, …). Paths não registrados MUST seguir o tratamento genérico de rotas inexistentes, sem handlers de compatibilidade por prefixo. URLs de mídia embutidas em eventos SHALL usar o path sem prefixo.

#### Scenario: Rota na raiz

- **WHEN** o cliente autenticado chama a rota sem prefixo
- **THEN** a operação é processada normalmente

#### Scenario: URL de mídia sem prefixo

- **WHEN** um evento referencia mídia armazenada
- **THEN** a URL usa o path sem prefixo e o download naquela URL funciona

#### Scenario: Path não registrado

- **WHEN** um cliente autorizado chama um path não registrado
- **THEN** recebe a resposta genérica de rota inexistente, sem redirecionamento ou adaptação de path

### Requirement: Logs do contrato vigente

Registros SHALL ser estruturados em JSON por padrão com alternativa legível para desenvolvimento, e SHALL ser correlacionáveis por requisição e instância; respostas MUST carregar identificador de requisição. Os únicos formatos aceitos SHALL ser json e console; qualquer outro MUST falhar na configuração sem alias ou conversão. O nível SHALL ser selecionável entre `debug`, `info`, `warn` e `error` com padrão `info`. Skips esperados de dados externos e retries de dependência MUST NOT gerar alerta acionável isolado. Registros de `Warn` e `Error` MUST conter apenas identificadores opacos (`instance_id`, `conversation_id`, `message_id`, `event_id`); dados pessoais e segredos MUST NOT aparecer nesses níveis.

#### Scenario: Rastreio de requisição

- **WHEN** uma requisição é processada
- **THEN** os registros e a resposta compartilham o mesmo identificador de requisição

#### Scenario: Seleção de formato e nível

- **WHEN** o operador configura formato `json` ou `console` e nível `debug`, `info`, `warn` ou `error`
- **THEN** o serviço emite registros no formato escolhido a partir do nível configurado, com `json` e `info` como padrões

#### Scenario: Skip esperado sem falso-positivo

- **WHEN** chega dado externo não espelhável (tipo não suportado, correlação ausente, inbox não provisionada) ou um aviso de pareamento sem QR
- **THEN** o serviço registra no máximo em nível informativo/debug com motivo tipado e segue operando sem alerta acionável

#### Scenario: Dependência indisponível sem flood

- **WHEN** banco ou broker fica inacessível durante retries em loop
- **THEN** o serviço limita a repetição de avisos e sinaliza o estado via prontidão em vez de um alerta por linha de log

#### Scenario: Probes fora do access log

- **WHEN** probes de liveness/prontidão ou assets de documentação/console são servidos
- **THEN** eles não geram linha informativa de access log por requisição

#### Scenario: PII e segredos minimizados

- **WHEN** qualquer operação registra aviso ou erro
- **THEN** telefone, JID, vcard, QR, token, apikey, cookie e senha nunca aparecem nesses registros

#### Scenario: Formato não suportado

- **WHEN** o operador informa um formato diferente de json ou console
- **THEN** a inicialização falha nomeando WZAP_LOG_FORMAT

## REMOVED Requirements

### Requirement: Rotas na raiz sem prefixo

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Rotas do contrato vigente na raiz".

**Migration:** usar somente a instalação nova e o requisito "Rotas do contrato vigente na raiz", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Rotas do contrato vigente na raiz" sem depender de cenários históricos deste requisito

### Requirement: Observabilidade mínima

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Logs do contrato vigente".

**Migration:** usar somente a instalação nova e o requisito "Logs do contrato vigente", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Logs do contrato vigente" sem depender de cenários históricos deste requisito
