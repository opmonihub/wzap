## MODIFIED Requirements

### Requirement: Observabilidade mínima

Registros SHALL ser estruturados em JSON por padrão com alternativa legível para desenvolvimento, e SHALL ser correlacionáveis por requisição e instância; respostas MUST carregar identificador de requisição. O nível SHALL ser selecionável entre `debug`, `info`, `warn` e `error` com padrão `info`. Skips esperados de dados externos e retries de dependência MUST NOT gerar alerta acionável isolado. Registros de `Warn` e `Error` MUST conter apenas identificadores opacos (`instance_id`, `conversation_id`, `message_id`, `event_id`); dados pessoais e segredos MUST NOT aparecer nesses níveis.

#### Scenario: Rastreio de requisição

- **WHEN** uma requisição é processada
- **THEN** os registros e a resposta compartilham o mesmo identificador de requisição

#### Scenario: Seleção de formato e nível

- **WHEN** o operador configura formato `json` ou `console` e nível `debug`, `info`, `warn` ou `error`
- **THEN** o serviço emite registros no formato escolhido a partir do nível configurado, com `json` e `info` como padrões

#### Scenario: Alias legado de formato

- **WHEN** o operador configura o formato legado `text`
- **THEN** o serviço inicia normalmente emitindo o formato legível de desenvolvimento

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
