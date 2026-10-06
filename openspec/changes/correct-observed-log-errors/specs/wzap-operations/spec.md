## ADDED Requirements

### Requirement: Persistência do JetStream local

O broker iniciado pelo compose local SHALL gravar o store do JetStream no volume já montado para o serviço, e MUST NOT usar diretório temporário do container. Reiniciar só o container do broker SHALL preservar o stream já criado nesse volume.

#### Scenario: Store no volume

- **WHEN** o ambiente local sobe o broker
- **THEN** o diretório de store do JetStream é o volume montado, e o log de boot não avisa que o storage é temporário

#### Scenario: Restart do broker

- **WHEN** o container do broker é recriado com o mesmo volume
- **THEN** o stream existente nesse volume continua disponível para o serviço
