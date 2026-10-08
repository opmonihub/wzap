## ADDED Requirements

### Requirement: Configuração opcional do frontend dev

O serviço SHALL aceitar `WZAP_MANAGER_DEV_URL` como endereço HTTP ou HTTPS absoluto do frontend dev, sem credenciais, query, fragmento ou prefixo de caminho além de `/`. Valor vazio SHALL manter o atendimento estático atual e valor inválido SHALL impedir a inicialização com erro que identifica a variável sem reproduzir seu conteúdo sensível.

#### Scenario: Variável ausente

- **WHEN** `WZAP_MANAGER_DEV_URL` está ausente ou vazia
- **THEN** o serviço utiliza o modo estático existente

#### Scenario: Endereço válido

- **WHEN** `WZAP_MANAGER_DEV_URL` contém um endereço HTTP ou HTTPS absoluto válido
- **THEN** o serviço habilita o encaminhamento do manager para esse endereço

#### Scenario: Endereço inválido

- **WHEN** `WZAP_MANAGER_DEV_URL` contém endereço relativo, protocolo não suportado, credenciais ou prefixo de caminho
- **THEN** a inicialização falha com erro que identifica a variável sem expor o valor informado

### Requirement: Troca exclusiva do modo local com dados preservados

Os modos compilado e dev documentados SHALL utilizar o mesmo projeto e o mesmo serviço de aplicação, substituindo a configuração dessa aplicação na troca de modo. A troca MUST preservar infraestrutura e volumes existentes e MUST NOT iniciar simultaneamente duas réplicas sobre o mesmo banco. O frontend dev SHALL iniciar como parte normal do ambiente dev e SHALL ficar acessível apenas pela rede interna dos serviços.

#### Scenario: Ativar desenvolvimento

- **WHEN** o operador ativa o modo dev pelo comando documentado sobre o ambiente compilado
- **THEN** a aplicação compilada é substituída pela aplicação dev, a entrada permanece em `127.0.0.1:8081` e a infraestrutura e seus volumes são reutilizados

#### Scenario: Retornar ao modo compilado

- **WHEN** o operador executa o retorno documentado ao modo compilado
- **THEN** o Go dev encerra antes do início do Go compilado e os dados existentes são preservados

#### Scenario: Projeto dev legado existente

- **WHEN** existe uma aplicação do antigo projeto dev sobre o mesmo banco antes da primeira ativação
- **THEN** ela deve estar encerrada antes de iniciar a aplicação substituta, sem remoção dos seus volumes

#### Scenario: Frontend dev iniciado com a aplicação

- **WHEN** o ambiente dev é iniciado pelo comando documentado
- **THEN** o frontend dev também inicia sem seleção de perfil extra e sem publicar uma porta própria no host

### Requirement: Recarga automática do backend sem sobreposição

O ambiente dev SHALL observar o código local e refletir uma alteração Go compilável mediante build e reinício automáticos dentro do mesmo container. Cada execução substituta SHALL começar apenas depois do encerramento da anterior, respeitando o limite de encerramento do serviço de 10 segundos; a parada do container SHALL permitir esse encerramento sem duplicar o sinal de término. Erros de compilação SHALL ser visíveis e a correção do código SHALL permitir recuperação automática.

#### Scenario: Alteração Go compilável

- **WHEN** o desenvolvedor salva uma alteração Go compilável
- **THEN** a aplicação reinicia automaticamente e atende usando o novo código sem reconstrução manual da imagem ou recriação do container

#### Scenario: Encerramento de oito segundos

- **WHEN** uma execução controlada leva 8 segundos para terminar após o pedido de recarga
- **THEN** a execução seguinte começa somente depois de sua saída, sem duas execuções ativas simultaneamente

#### Scenario: Edições sucessivas durante encerramento

- **WHEN** novas alterações chegam enquanto a execução anterior encerra
- **THEN** os reinícios não se sobrepõem e a versão compilável mais recente acaba sendo atendida

#### Scenario: Parada durante encerramento

- **WHEN** o container é parado com uma execução ainda encerrando dentro do limite do serviço
- **THEN** a execução termina antes da saída do supervisor, sem aborto por sinal de término repetido

#### Scenario: Código inválido corrigido

- **WHEN** uma alteração causa erro de compilação e depois é corrigida
- **THEN** o erro é apresentado e o ambiente retoma automaticamente a execução do código corrigido sem ação manual de build ou restart
