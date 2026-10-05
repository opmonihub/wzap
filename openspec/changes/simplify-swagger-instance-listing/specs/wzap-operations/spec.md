## MODIFIED Requirements

### Requirement: Documentação interativa da API

A documentação servida SHALL descrever as rotas, corpos, envelopes de resposta e o esquema `apikey`, permitindo testar chamadas com uma key informada pelo leitor uma única vez no Authorize global da página. A documentação MUST refletir as rotas efetivamente servidas. Operações MUST NOT declarar campos de entrada manuais `apikey` ou `X-Request-Id`; o serviço SHALL continuar exigindo credencial nas rotas protegidas e gerando ou propagando o identificador de correlação na resposta.

#### Scenario: Testar chamada pela documentação

- **WHEN** o leitor informa uma apikey válida no Authorize e executa uma chamada protegida
- **THEN** a chamada é enviada com o header `apikey:` sem exigir nova entrada da chave e a resposta é exibida

#### Scenario: Correlação automática

- **WHEN** o leitor abre os parâmetros de uma operação
- **THEN** nenhum campo de entrada X-Request-Id aparece e a resposta da chamada ainda contém o identificador de correlação
