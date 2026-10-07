## MODIFIED Requirements

### Requirement: Armazenamento com integridade

A mídia armazenada MUST registrar tipo, tamanho, nome quando houver e checksum, permitindo verificar integridade. Os bytes SHALL ser armazenados no MinIO e identificados por bucket e chave de objeto únicos; referências à mídia SHALL conservar seu UUID e escopo de instância. Migração de arquivos locais MUST verificar conteúdo antes de trocar a referência persistida.

#### Scenario: Mídia armazenada

- **WHEN** uma mídia é armazenada
- **THEN** seus metadados e checksum ficam registrados e o objeto correspondente pode ser lido pelo fluxo autorizado

#### Scenario: Arquivo legado migrado

- **WHEN** um arquivo local existente é transferido para o MinIO
- **THEN** conserva UUID, tamanho e checksum após verificação, mantendo as referências de mensagens

### Requirement: Expiração e limpeza

Mídias MUST expirar após o período configurado. Objetos expirados SHALL ser removidos periodicamente, preservando metadados e vínculos das mensagens; object_deleted_at SHALL registrar a confirmação da exclusão do objeto. Falhas de exclusão MUST permanecer recuperáveis para nova tentativa.

#### Scenario: Mídia expirada

- **WHEN** o período de retenção termina
- **THEN** a mídia deixa de estar disponível para download e seu objeto é removido, com metadados e referências preservados

#### Scenario: Falha ao excluir objeto

- **WHEN** o MinIO não confirma a exclusão
- **THEN** o registro continua pendente de limpeza e pode ser tentado novamente

## ADDED Requirements

### Requirement: Infraestrutura de mídia aprovada

A integração SHALL usar exatamente a imagem quay.io/minio/minio:RELEASE.2024-01-13T07-53-03Z-cpuv1, a rede existente do projeto e credenciais configuradas pelo ambiente. A disponibilidade da imagem MUST ser validada antes do teste de integração e da implantação; nenhuma tag alternativa está autorizada por este registro.

#### Scenario: Imagem solicitada indisponível

- **WHEN** a imagem exata não pode ser obtida e verificada
- **THEN** a implantação fica pendente sem substituição silenciosa por outra imagem
