## ADDED Requirements

### Requirement: Erros de restauração sem identificador interno de dispositivo

Logs e erros persistidos/publicados da restauração SHALL usar identificadores opacos e descrições seguras. JID de dispositivo MUST NOT aparecer no log nem em connection.last_error, inclusive para parse, ausência, mismatch ou falha upstream.

#### Scenario: Vínculo inválido ou ausente
- **WHEN** a restauração falha ao interpretar ou encontrar o dispositivo vinculado
- **THEN** a instância informa o erro útil sem publicar o JID interno em log ou resposta

### Requirement: Build Go sem assets da console

Clone do projeto SHALL permitir build/test Go antes da geração Nuxt usando o placeholder de embed. A console não construída SHALL continuar respondendo 503; assets gerados MUST NOT ser necessários como arquivos versionados.

#### Scenario: Clone sem build Nuxt
- **WHEN** o clone não possui index.html nem bundle da console
- **THEN** build/test Go compila e a console indica indisponibilidade com 503
