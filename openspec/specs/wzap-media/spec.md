# wzap-media Specification

## Purpose

Armazenar temporariamente e servir mídias recebidas e enviadas com integridade, autenticação e expiração.

## Requirements

### Requirement: Download automático de mídia recebida

Mensagens com mídia SHALL ter o conteúdo baixado automaticamente até o limite configurado; acima do limite, o evento MUST indicar omissão da mídia sem interromper o fluxo.

#### Scenario: Mídia dentro do limite

- **WHEN** chega mensagem com mídia de tamanho dentro do limite
- **THEN** o conteúdo é armazenado e referenciado no evento

#### Scenario: Mídia acima do limite

- **WHEN** chega mídia maior que o limite configurado
- **THEN** o evento é publicado com indicação de mídia omitida

### Requirement: Armazenamento com integridade

A mídia armazenada MUST registrar tipo, tamanho, nome quando houver e checksum, permitindo verificar integridade. Os bytes SHALL ser armazenados no MinIO e identificados por bucket e chave de objeto únicos; referências à mídia SHALL conservar seu UUID e escopo de instância. Migração de arquivos locais MUST verificar conteúdo antes de trocar a referência persistida.

#### Scenario: Mídia armazenada

- **WHEN** uma mídia é armazenada
- **THEN** seus metadados e checksum ficam registrados e o objeto correspondente pode ser lido pelo fluxo autorizado

#### Scenario: Arquivo legado migrado

- **WHEN** um arquivo local existente é transferido para o MinIO
- **THEN** conserva UUID, tamanho e checksum após verificação, mantendo as referências de mensagens

### Requirement: Download autenticado

O acesso à mídia SHALL exigir autenticação de serviço; mídia inexistente ou expirada MUST responder `404`; a resposta MUST informar o tipo de conteúdo correto.

#### Scenario: Download válido

- **WHEN** o cliente autenticado baixa uma mídia existente
- **THEN** recebe o conteúdo com o tipo correto

#### Scenario: Acesso sem credencial

- **WHEN** o acesso ocorre sem token válido
- **THEN** a resposta é `401`

### Requirement: Expiração e limpeza

Mídias MUST expirar após o período configurado. Objetos expirados SHALL ser removidos periodicamente, preservando metadados e vínculos das mensagens; object_deleted_at SHALL registrar a confirmação da exclusão do objeto. Falhas de exclusão MUST permanecer recuperáveis para nova tentativa.

#### Scenario: Mídia expirada

- **WHEN** o período de retenção termina
- **THEN** a mídia deixa de estar disponível para download e seu objeto é removido, com metadados e referências preservados

#### Scenario: Falha ao excluir objeto

- **WHEN** o MinIO não confirma a exclusão
- **THEN** o registro continua pendente de limpeza e pode ser tentado novamente

### Requirement: Upload para envio

Mídias enviadas por upload MUST ser validadas quanto a tipo permitido e tamanho antes do enfileiramento; inválidas MUST responder `422`.

#### Scenario: Upload válido

- **WHEN** o cliente envia arquivo permitido dentro do limite
- **THEN** a mensagem é enfileirada referenciando a mídia armazenada

### Requirement: Identificadores opacos

Identificadores de mídia MUST ser opacos e não sequenciais, dificultando acesso não autorizado.

#### Scenario: Identificador não adivinhável

- **WHEN** uma mídia é criada
- **THEN** seu identificador não revela sequência nem conteúdo previsível

### Requirement: Infraestrutura de mídia aprovada

A integração SHALL usar exatamente a imagem docker.io/cccs/minio fixada ao digest sha256:68eefa6a5ccd82178a872b2d1012687d0ac9b1afa848a1e56f55fa5f1efcb081, a rede existente do projeto e credenciais configuradas pelo ambiente. Essa referência fixada por digest SUPERSEDE a tag quay.io/minio/minio citada em registros anteriores desta capability. A disponibilidade da imagem MUST ser validada antes do teste de integração e da implantação; nenhuma tag alternativa está autorizada por este registro.

#### Scenario: Imagem solicitada indisponível

- **WHEN** a imagem exata não pode ser obtida e verificada
- **THEN** a implantação fica pendente sem substituição silenciosa por outra imagem
