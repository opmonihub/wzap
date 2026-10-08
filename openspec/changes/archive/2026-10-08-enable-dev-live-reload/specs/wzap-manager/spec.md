## ADDED Requirements

### Requirement: Atualização automática do manager em desenvolvimento

O manager em desenvolvimento SHALL carregar em `/manager/` na mesma origem da API e SHALL refletir edições locais de componentes e estilos por atualização automática, sem gerar o bundle de produção, reconstruir imagens ou reiniciar containers manualmente. O navegador MUST estabelecer HMR pela mesma entrada pública e MUST NOT depender de uma porta frontend publicada separadamente.

#### Scenario: Alterar um componente

- **WHEN** o desenvolvedor salva uma alteração de componente com o ambiente dev ativo
- **THEN** o navegador recebe a atualização pela origem pública do manager sem compilação ou reinício manual

#### Scenario: Alterar estilos

- **WHEN** o desenvolvedor salva uma alteração de CSS com o manager aberto
- **THEN** os estilos atualizados aparecem automaticamente sem recriação do container

#### Scenario: Autenticação pela mesma origem

- **WHEN** o usuário acessa `/manager/`, faz login e consulta uma rota interna em desenvolvimento
- **THEN** o painel e suas chamadas à API usam a mesma origem e a sessão por cookie existente

#### Scenario: Restabelecer HMR após recarga do backend

- **WHEN** uma recarga automática do backend interrompe a conexão HMR
- **THEN** o navegador restabelece a conexão pela entrada pública e uma nova edição do frontend volta a ser refletida

### Requirement: Manager embutido preservado em produção

Sem configuração de encaminhamento dev, o serviço SHALL preservar o atendimento estático atual do manager, incluindo assets, navegação direta e refresh. A imagem de produção SHALL carregar o manager embutido sem depender de um processo Nuxt em runtime.

#### Scenario: Imagem de produção sem Nuxt

- **WHEN** a imagem compilada inicia sem um serviço Nuxt dev
- **THEN** `/manager/`, uma rota interna e seus assets são atendidos pelo próprio serviço

#### Scenario: Clone sem bundle e sem modo dev

- **WHEN** o serviço executa sem encaminhamento dev e sem bundle utilizável
- **THEN** o manager continua indicando indisponibilidade com `503`, preservando o build Go permitido pelo placeholder
