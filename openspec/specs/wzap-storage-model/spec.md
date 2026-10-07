# wzap-storage-model Specification

## Purpose

Definir a preservação dos dados e as relações do modelo persistido aprovado, para que a remodelagem mantenha identidades, isolamento de instâncias e integridade verificável.

## Requirements

### Requirement: Identidades preservadas durante a remodelagem

A migração SHALL preservar os UUIDs existentes, referências externas, ownership e dados recuperáveis no modelo de 14 tabelas aprovado no design. Tabelas com chaves naturais ou compostas SHALL receber UUID próprio preservando a unicidade anterior. Schemas de bibliotecas e do migrador MUST permanecer sob seus contratos existentes.

#### Scenario: Upgrade com dados existentes

- **WHEN** um banco com instâncias, mensagens, mídias e correlações existentes é migrado
- **THEN** seus UUIDs e associações continuam identificando os mesmos recursos e nenhuma chave natural perde a garantia de unicidade

### Requirement: Relações isoladas por instância

Uma mensagem SHALL referenciar opcionalmente uma mídia da mesma instância; uma mídia SHALL poder atender vários envios. Uma correlação Chatwoot SHALL referenciar opcionalmente um envio da mesma instância sem exigir registros de fila para entradas ou edições.

#### Scenario: Referência a mídia de outra instância

- **WHEN** é solicitado um vínculo entre uma mensagem e mídia de outra instância
- **THEN** o vínculo é rejeitado sem alterar os registros existentes

#### Scenario: Entrada sem fila

- **WHEN** uma mensagem recebida é espelhada no Chatwoot
- **THEN** sua correlação permanece válida sem criar uma mensagem artificial na fila de saída

### Requirement: Migração verificável sem fabricação de dados

A migração MUST detectar referências órfãs, dispositivos divergentes e estados incompatíveis antes do corte e MUST preservar os dados para tratamento explícito. Datas de erro desconhecidas e identificadores externos ausentes MUST NOT ser fabricados. Uma divergência entre identidades de dispositivo (`whatsapp_jid` vs `device_jid`) SHALL ser registrada no relatório de auditoria com ambos os valores e MUST bloquear o corte: o comando de migração e o boot com migração automática MUST falhar enquanto a divergência não tiver resolução explícita registrada no relatório. A resolução MUST permanecer no relatório como histórico de auditoria, com a marca de quando foi registrada.

#### Scenario: Vínculo antigo não recuperável

- **WHEN** a auditoria encontra uma referência de mídia cujo registro e objeto já foram removidos
- **THEN** o corte não prossegue sem a política documentada de tratamento e nenhum checksum ou arquivo fictício é criado

#### Scenario: Divergência de identidade bloqueia o corte

- **WHEN** a migração registra divergência entre `whatsapp_jid` e `device_jid` e a migração completa é executada sem resolução explícita
- **THEN** o comando de migração e o boot com migração automática falham nomeando os casos divergentes, o banco permanece na versão anterior ao gate e a row de divergência é preservada no relatório

#### Scenario: Resolução explícita libera o corte

- **WHEN** o operador reconcilia a identidade e marca a divergência como resolvida no relatório de auditoria
- **THEN** a migração completa atravessa o gate e a row de divergência permanece no relatório com ambos os valores e a marca de resolução