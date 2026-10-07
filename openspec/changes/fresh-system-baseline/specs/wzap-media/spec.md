## MODIFIED Requirements

### Requirement: Expiração e limpeza

Mídias MUST expirar após o período configurado. Conteúdos expirados SHALL ser removidos periodicamente, preservando metadados e vínculos das mensagens; object_deleted_at SHALL registrar a confirmação da exclusão do conteúdo. Falhas de exclusão MUST permanecer recuperáveis para nova tentativa.

#### Scenario: Mídia expirada

- **WHEN** o período de retenção termina
- **THEN** a mídia deixa de estar disponível para download e seus bytes são removidos, com metadados e referências preservados

#### Scenario: Falha ao excluir objeto

- **WHEN** o backend selecionado não confirma a exclusão
- **THEN** o registro continua pendente de limpeza e pode ser tentado novamente

## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Armazenamento com integridade

**Reason:** a base nova elimina cenários e garantias de compatibilidade histórica deste requisito; o comportamento vigente fica definido em "Integridade de mídia no backend selecionado".

**Migration:** usar somente a instalação nova e o requisito "Integridade de mídia no backend selecionado", sem adaptação ou importação de dados anteriores.

#### Scenario: Contrato vigente na instalação nova

- **WHEN** a funcionalidade é utilizada na instalação nova
- **THEN** segue "Integridade de mídia no backend selecionado" sem depender de cenários históricos deste requisito
