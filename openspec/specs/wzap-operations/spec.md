# wzap-operations Specification

## Purpose

Definir autenticação, configuração, saúde, migrações e execução do serviço no ambiente local.

## Requirements

### Requirement: Autenticação de serviço

Todos os endpoints da API, exceto saúde, prontidão, console e documentação, SHALL exigir credencial válida: sessão de usuário OU header `apikey:`. Credencial ausente ou inválida MUST responder `401` sem revelar detalhes; credencial válida sem escopo para a operação MUST responder `403`.

#### Scenario: Requisição autenticada

- **WHEN** o cliente envia sessão válida ou apikey válida com escopo
- **THEN** a requisição é processada

#### Scenario: Token inválido

- **WHEN** o cliente envia credencial ausente ou inválida (incluindo o esquema legado `Authorization: Bearer`)
- **THEN** a resposta é `401` sem detalhes que permitam inferir credencial válida

#### Scenario: Sem escopo

- **WHEN** o cliente envia credencial válida para operação fora do seu escopo
- **THEN** a resposta é `403`

### Requirement: Rotas na raiz sem prefixo

As rotas da API SHALL viver na raiz, sem prefixo de versão (`/instances`, `/instances/{id}/messages/text`, `/media/{id}`, …). O prefixo `/api/v1` anterior MUST NOT ser mais atendido. URLs de mídia embutidas em eventos SHALL usar o path sem prefixo.

#### Scenario: Rota na raiz

- **WHEN** o cliente autenticado chama a rota sem prefixo
- **THEN** a operação é processada normalmente

#### Scenario: Prefixo legado

- **WHEN** o cliente chama qualquer rota sob `/api/v1`
- **THEN** a resposta é `404`, como rota inexistente

#### Scenario: URL de mídia sem prefixo

- **WHEN** um evento referencia mídia armazenada
- **THEN** a URL usa o path sem prefixo e o download naquela URL funciona

### Requirement: Superfícies servidas

O serviço SHALL servir o console em `/manager` e a documentação interativa em `/swagger/*`, ambas sem exigir credencial para carregar; as chamadas de dados seguem as regras de autenticação de cada plano.

#### Scenario: Console carrega sem credencial

- **WHEN** o navegador abre `/manager` sem sessão
- **THEN** o aplicativo carrega e apresenta o login

#### Scenario: Documentação carrega sem credencial

- **WHEN** o cliente abre `/swagger/` sem credencial
- **THEN** a documentação interativa carrega

### Requirement: Documentação interativa da API

A documentação servida SHALL descrever cada par método/path explicitamente registrado, seus parâmetros, corpos, códigos de resposta e o esquema `apikey`, permitindo testar chamadas com uma key informada pelo leitor uma única vez no Authorize global da página. A documentação MUST refletir as rotas efetivamente servidas e suas regras de acesso, incluindo a alternativa de sessão por cookie nas rotas que a aceitam. Operações MUST NOT declarar campos de entrada manuais `apikey` ou `X-Request-Id`; o serviço SHALL continuar exigindo credencial nas rotas protegidas e gerando ou propagando o identificador de correlação na resposta. Os schemas SHALL representar o envelope `data` das respostas JSON padronizadas e o envelope `error` dos erros padronizados; respostas sem corpo, downloads e superfícies públicas com formatos próprios SHALL preservar seus formatos reais.

#### Scenario: Testar chamada pela documentação

- **WHEN** o leitor informa uma apikey válida no Authorize e executa uma chamada protegida
- **THEN** a chamada é enviada com o header `apikey:` sem exigir nova entrada da chave e a resposta é exibida

#### Scenario: Correlação automática

- **WHEN** o leitor abre os parâmetros de uma operação
- **THEN** nenhum campo de entrada X-Request-Id aparece e a resposta da chamada ainda contém o identificador de correlação

#### Scenario: Consultar qualquer operação registrada

- **WHEN** o leitor consulta o Swagger de uma operação HTTP explicitamente registrada, incluindo configuração, importação e comandos Chatwoot e a entrada do Manager
- **THEN** encontra o método, path, parâmetros e respostas da operação
- **AND** o webhook Chatwoot e as entradas públicas não exigem apikey na documentação

#### Scenario: Usar o schema de uma resposta JSON

- **WHEN** o leitor consulta o schema de uma resposta que a API entrega com envelope
- **THEN** o schema contém `data` ou `error` conforme a resposta real, incluindo o tipo dos itens de coleções

#### Scenario: Consultar respostas especiais

- **WHEN** o leitor consulta uma resposta 204, download de mídia, readiness indisponível, webhook Chatwoot ou entrada do Manager
- **THEN** a documentação representa respectivamente ausência de corpo, conteúdo binário, envelope `data`, objeto próprio do webhook ou redirecionamento/conteúdo público

### Requirement: Saúde e prontidão

O serviço SHALL expor liveness (processo vivo) e readiness (dependências essenciais prontas). Readiness MUST responder indisponível enquanto dependências ou migrações não estiverem prontas.

#### Scenario: Dependência indisponível

- **WHEN** o banco ou o broker estiver inacessível
- **THEN** readiness responde indisponível e liveness continua respondendo vivo

### Requirement: Configuração por ambiente

O serviço SHALL ser configurável por variáveis de ambiente e MUST falhar na inicialização, com erro claro, quando configuração obrigatória estiver ausente ou inválida.

#### Scenario: Configuração ausente

- **WHEN** uma variável obrigatória não é informada
- **THEN** a inicialização falha com mensagem identificando a variável

### Requirement: Migrações

O serviço SHALL aplicar migrações de esquema quando habilitado e MUST refletir o estado das migrações na prontidão.

#### Scenario: Migrações pendentes

- **WHEN** existem migrações não aplicadas e a aplicação automática está habilitada
- **THEN** elas são aplicadas antes de o serviço aceitar tráfego

### Requirement: Empacotamento

A imagem do serviço SHALL executar como usuário não privilegiado, conter apenas o necessário para executar o serviço e suportar verificação de saúde sem ferramentas externas.

#### Scenario: Execução como não-root

- **WHEN** o contêiner inicia
- **THEN** o processo executa sem privilégios de root

### Requirement: Execução local

Subir o ambiente SHALL iniciar o serviço junto das dependências de banco e broker, persistindo dados em volume.

#### Scenario: Ambiente completo

- **WHEN** o ambiente local é iniciado
- **THEN** o serviço fica pronto e apto a receber requisições autenticadas

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

### Requirement: Encerramento gracioso

Ao receber sinal de término, o serviço SHALL parar de aceitar novas requisições e concluir ou liberar o trabalho em andamento dentro de um tempo limitado.

#### Scenario: Sinal de término

- **WHEN** o serviço recebe pedido de encerramento
- **THEN** ele para de aceitar requisições e finaliza o processamento em andamento no prazo definido

### Requirement: Persistência do JetStream local

O broker iniciado pelo compose local SHALL gravar o store do JetStream no volume já montado para o serviço, e MUST NOT usar diretório temporário do container. Reiniciar só o container do broker SHALL preservar o stream já criado nesse volume.

#### Scenario: Store no volume

- **WHEN** o ambiente local sobe o broker
- **THEN** o diretório de store do JetStream é o volume montado, e o log de boot não avisa que o storage é temporário

#### Scenario: Restart do broker

- **WHEN** o container do broker é recriado com o mesmo volume
- **THEN** o stream existente nesse volume continua disponível para o serviço

### Requirement: Authorization and documentation for instance aliases

Swagger SHALL describe instance path parameters as UUID or name and document targeted stats, validation and conflict responses. Aliases MUST preserve existing authorization; an instance key MUST only operate its own current name or UUID. Idempotent replay MUST recheck current access before returning a cached response, and using a UUID/name alias with the same key and request MUST share the canonical instance's replay.

#### Scenario: Swagger name input
- **WHEN** a reader opens an instance-scoped operation or stats
- **THEN** the documentation explains UUID/name references and exposes the optional stats target

#### Scenario: Alias replay
- **WHEN** an authorized client repeats an identical idempotent request and key using the other reference for the same instance
- **THEN** the previous response is replayed without repeating the operation

#### Scenario: Access revoked before replay
- **WHEN** a client lacks current ownership or uses a foreign instance key against a cached operation
- **THEN** the response is 403 and the cached response is not exposed
