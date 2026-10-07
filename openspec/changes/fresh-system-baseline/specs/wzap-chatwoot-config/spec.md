## MODIFIED Requirements

### Requirement: Configuração por instância

Cada instância SHALL ter configuração própria gerenciável por quem pode operá-la. **BREAKING**: os nomes curtos aprovados são is_enabled, url, account_id, token, inbox_name, is_sign_enabled, sign_delimiter, is_reopen_enabled, is_pending_enabled, is_merge_enabled, is_import_contacts, is_import_messages, import_days, is_auto_create, organization, logo e ignored_jids. Ao habilitar, URL/account/token válidos SHALL ser exigidos; flags não booleanas MUST ser rejeitadas com 422 sem alterar a configuração anterior. HTTP SHALL exigir host loopback; demais hosts SHALL exigir HTTPS. Inbox vazia SHALL assumir o identificador da instância e delimitador vazio SHALL assumir quebra de linha. Tokens não vazios MUST ser persistidos exclusivamente com proteção AES-256-GCM; leitura MUST validar e decifrar esse formato sem aceitar texto puro ou converter dados históricos. Token MUST NOT ser devolvido nas respostas.

#### Scenario: Habilitar conector

- **WHEN** o operador grava configuração válida com is_enabled true
- **THEN** o espelho e o webhook da instância passam a operar

#### Scenario: Config inválida

- **WHEN** o operador grava URL malformada ou omite account_id/token ao habilitar
- **THEN** recebe 422 e a configuração anterior é mantida

#### Scenario: Leitura sem configuração

- **WHEN** a instância nunca foi configurada
- **THEN** a leitura informa integração desligada sem expor token

## ADDED Requirements

### Requirement: Chave de cifragem obrigatória para o conector

**BREAKING:** com WZAP_CHATWOOT_ENABLED=true, WZAP_CHATWOOT_TOKEN_KEY MUST estar presente e representar exatamente 32 bytes em base64; ausência ou valor inválido MUST falhar o boot nomeando a variável. Tokens não vazios MUST NOT ser persistidos sem chave válida, independentemente do estado da configuração por instância. Tokens armazenados adulterados ou indecifráveis MUST produzir erro sem exposição do segredo. Configurações sem token SHALL continuar admitidas quando desligadas.

#### Scenario: Conector sem chave

- **WHEN** Chatwoot está habilitado globalmente e a chave não foi configurada
- **THEN** o serviço falha na inicialização identificando WZAP_CHATWOOT_TOKEN_KEY

#### Scenario: Chave malformada

- **WHEN** uma chave configurada não representa 32 bytes válidos
- **THEN** a inicialização falha sem imprimir o valor da chave

#### Scenario: Token cifrado

- **WHEN** uma configuração válida com token é persistida com chave válida
- **THEN** o banco não armazena o token em texto puro e as leituras públicas não o expõem

#### Scenario: Adulteração

- **WHEN** a leitura encontra um token armazenado que não pode ser autenticado e decifrado
- **THEN** informa falha sem devolver o conteúdo como texto puro

#### Scenario: Conector desligado sem token

- **WHEN** Chatwoot está desligado e não há token configurado
- **THEN** a ausência de chave não impede o funcionamento das demais funcionalidades
