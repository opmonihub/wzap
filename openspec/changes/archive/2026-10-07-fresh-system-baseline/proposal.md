## Why

O projeto foi aprovado para começar com dados vazios, mas ainda carrega migrações, conversores, backfills e exceções para versões anteriores; duas migrações `00009` inclusive impedem a instalação do código atual. Uma base inicial única e um contrato vigente único eliminam essa dependência histórica sem retirar as funcionalidades atuais do gateway.

## What Changes

- **BREAKING:** suportar instalação nova sobre banco vazio, substituindo a cadeia histórica por uma migração inicial do schema atual de 15 tabelas próprias; remover remodelagem, relatórios, backfills e gates de corte.
- Exigir ownership e nomes válidos globalmente únicos; retirar exceções para instâncias antigas e preservar UUID/nome exato, autorização, quotas e isolamento.
- **BREAKING:** remover conversão de respostas idempotentes antigas, `legacy_error`, armazenamento de tokens Chatwoot em texto puro e o alias de log `text` e campos reservados exclusivamente para compatibilidade futura; exigir chave de cifragem quando Chatwoot estiver habilitado.
- **BREAKING:** remover `media-migrate` e transferências de mídia antiga; oferecer S3/MinIO e disco como backends oficiais para dados novos, selecionados pela presença de `WZAP_S3_ENDPOINT`, sem fallback em indisponibilidade de S3.
- Retirar tratamento dedicado de prefixos e configurações antigos, limpeza vestigial de eventos já publicados e testes/fixtures exclusivos de compatibilidade; manter testes de segurança, falhas, retries e contrato atual.
- Alinhar Manager, Swagger, README e instruções vigentes ao sistema novo; preservar eventos v1, correlações Chatwoot pendentes e funcionalidades atuais.
- Na aplicação futura, resetar o ambiente local sem backup após os gates: descartar banco, sessões, mensagens, mídia e eventos, reconstruir e validar uma única réplica.

## Capabilities

### New Capabilities

Nenhuma.

### Modified Capabilities

- `wzap-storage-model`: instalação limpa, relações do schema atual, ownership obrigatório e remoção de preservação/conversão de dados históricos.
- `wzap-accounts`: seed exclusivo do primeiro admin, sem adoção de instâncias antigas.
- `wzap-api-keys`: credenciais do contrato atual e rotação após revogação, sem backfill histórico.
- `wzap-instances`: nomes válidos únicos e identidade exclusiva sem exceções para dados anteriores.
- `wzap-response-contract`: erros atuais estruturados e replay do resultado armazenado sem conversão de envelopes.
- `wzap-message-lifecycle`: resposta de revogação sem campo reservado exclusivamente para compatibilidade futura.
- `wzap-outbound-messaging`: idempotência exclusiva do contrato vigente, mantendo ausência de efeitos duplicados.
- `wzap-media`: S3 ou disco para instalações novas, integridade, expiração e remoção da migração de arquivos.
- `wzap-chatwoot-config`: tokens cifrados obrigatoriamente e chave validada quando o conector está habilitado.
- `wzap-operations`: instalação, comandos, formatos de log e reset local sem backup.
- `wzap-manager`: formulários e mensagens do contrato vigente, sem exceções para nomes e erros históricos.

## Impact

- Go: schema/migrador, repositories, seed e wiring, instâncias, DTOs, idempotência, mídia, configuração/logger e relay de eventos; respectivos testes e fixtures.
- Manager: formulários de nome, tipos de erro, traduções e consumo do contrato atual; documentação Swagger regenerada após alterações de anotações.
- Operação futura: remoção apenas dos volumes `wzap_pgdata`, `wzap_natsdata`, `wzap_miniodata` e `wzap_media`; novo cadastro e pareamento serão necessários.
- Nenhuma dependência nova, mudança de workflows CI, réplica adicional ou mudança do envelope/event_id dos eventos v1.

## Out-of-Scope

- Esta proposta cria somente artefatos OpenSpec; não implementa código, não cria worktree de implementação, não reseta volumes e não faz deploy.
- Upgrade, conversão, importação, recuperação, backup ou restauração de dados de instalações anteriores.
- Reescrever funcionalidades atuais, remover importação operacional Chatwoot, eliminar correlação `pending:{uuid}`, retries ou suporte ativo de protocolo.
- Apagar histórico Git ou arquivos de changes arquivadas; incorporar alterações locais não relacionadas; introduzir domínio de tenant/account ou modificar schemas administrados pelo whatsmeow.
