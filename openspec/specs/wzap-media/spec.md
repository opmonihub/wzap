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

### Requirement: Download autenticado

O acesso à mídia SHALL exigir autenticação de serviço; mídia inexistente ou expirada MUST responder `404`; a resposta MUST informar o tipo de conteúdo correto.

#### Scenario: Download válido

- **WHEN** o cliente autenticado baixa uma mídia existente
- **THEN** recebe o conteúdo com o tipo correto

#### Scenario: Acesso sem credencial

- **WHEN** o acesso ocorre sem token válido
- **THEN** a resposta é `401`

### Requirement: Expiração e limpeza

Mídias MUST expirar após o período configurado. Conteúdos expirados SHALL ser removidos periodicamente, preservando metadados e vínculos das mensagens; object_deleted_at SHALL registrar a confirmação da exclusão do conteúdo. Falhas de exclusão MUST permanecer recuperáveis para nova tentativa.

#### Scenario: Mídia expirada

- **WHEN** o período de retenção termina
- **THEN** a mídia deixa de estar disponível para download e seus bytes são removidos, com metadados e referências preservados

#### Scenario: Falha ao excluir objeto

- **WHEN** o backend selecionado não confirma a exclusão
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

### Requirement: Seleção estável de backend

A seleção de backend SHALL ocorrer pela configuração no início do processo. S3 configurado MUST NOT ser substituído por disco em falhas de inicialização, leitura, gravação ou exclusão. Disco em modo S3 SHALL servir apenas como cache temporário para consumidores que precisam de arquivo. Transferência automática de registros entre backends MUST NOT existir.

#### Scenario: S3 indisponível no boot

- **WHEN** o endpoint S3 está configurado e o bucket não pode ser preparado
- **THEN** a inicialização falha com erro de mídia sem iniciar em modo local

#### Scenario: Falha de escrita S3

- **WHEN** o backend S3 falha durante uma gravação
- **THEN** a operação informa erro e não grava uma cópia persistente em disco como alternativa

#### Scenario: Cache temporário

- **WHEN** um envio em modo S3 precisa de um arquivo local
- **THEN** o arquivo é materializado do objeto como cache descartável sem mudar seu backend persistente

### Requirement: Integridade de mídia no backend selecionado

A mídia armazenada MUST registrar tipo, tamanho, nome quando houver, checksum, UUID e escopo de instância. Bytes SHALL ser armazenados no backend selecionado para a instalação: S3/MinIO quando WZAP_S3_ENDPOINT estiver configurado e disco local quando estiver ausente. Registros novos MUST identificar explicitamente bucket e chave de conteúdo; operações S3 MUST usar o bucket registrado, sem reconstrução de referências históricas.

#### Scenario: Mídia em S3

- **WHEN** uma mídia é criada em instalação com S3 configurado
- **THEN** seus bytes são gravados no bucket configurado e seus metadados, checksum e bucket ficam registrados

#### Scenario: Mídia em disco

- **WHEN** uma mídia é criada sem endpoint S3 configurado
- **THEN** seus bytes são gravados no diretório local configurado com metadados e checksum persistidos

#### Scenario: Bucket registrado

- **WHEN** o bucket padrão configurado muda e uma mídia da mesma instalação ainda referencia outro bucket
- **THEN** leitura e exclusão S3 usam o bucket registrado naquela mídia
