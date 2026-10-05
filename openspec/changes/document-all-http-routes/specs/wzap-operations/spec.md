## MODIFIED Requirements

### Requirement: Documentação interativa da API

A documentação servida SHALL descrever cada par método/path explicitamente registrado, seus parâmetros, corpos, códigos de resposta e o esquema `apikey`, permitindo testar chamadas com uma key informada pelo leitor. A documentação MUST refletir as rotas efetivamente servidas e suas regras de acesso, incluindo a alternativa de sessão por cookie nas rotas que a aceitam. Os schemas SHALL representar o envelope `data` das respostas JSON padronizadas e o envelope `error` dos erros padronizados; respostas sem corpo, downloads e superfícies públicas com formatos próprios SHALL preservar seus formatos reais.

#### Scenario: Testar chamada pela documentação

- **WHEN** o leitor informa uma apikey válida na documentação e executa uma chamada
- **THEN** a chamada é enviada com o header `apikey:` e a resposta é exibida

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
