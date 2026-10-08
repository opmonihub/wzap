## Why

O desenvolvimento atual usa um projeto Compose separado e o manager servido pelo Go depende de geração manual dos assets. Precisamos refletir alterações locais de Go e Nuxt automaticamente, mantendo a mesma entrada HTTP e uma única réplica sobre os dados existentes.

## What Changes

- Manter Go em `http://127.0.0.1:8081`, com REST na raiz, painel em `/manager/` e documentação em `/swagger/`.
- Adicionar `WZAP_MANAGER_DEV_URL`, opcional, para encaminhar o painel ao Nuxt dev, preservando paths, query strings, Host público e WebSocket/HMR; responder `503` quando Nuxt estiver indisponível.
- Ativar o manager dev junto do Go dev, com código local montado e HMR pela entrada pública, sem geração manual do bundle.
- Priorizar os recursos nativos do Air e comprovar encerramento seguro antes de adicionar qualquer wrapper mínimo.
- Converter o Compose dev em sobreposição do projeto principal, substituindo o mesmo serviço `wzap` e reutilizando infraestrutura e volumes.
- Documentar inicialização, retorno ao modo compilado e exceções que exigem reaplicar configuração ou dependências.

Não há mudança no contrato REST ou nos eventos; o comando dev muda de arquivo independente para composição do arquivo base com a sobreposição.

## Capabilities

### New Capabilities

Nenhuma.

### Modified Capabilities

- `wzap-manager`: atualização automática de interface e estilos em desenvolvimento, na mesma origem da API, com produção embutida preservada.
- `wzap-http-routing`: encaminhamento do namespace do manager em desenvolvimento, incluindo upgrade WebSocket e indisponibilidade explícita do Nuxt.
- `wzap-operations`: troca exclusiva entre Go compilado e dev, recarga automática com encerramento seguro e preservação da infraestrutura e dos dados locais.

## Impact

Configuração Go, composição HTTP, handler do manager, configuração Nuxt, Air, Dockerfile dev, Compose dev e documentação de uso. Testes devem cobrir HTTP e WebSocket reais, ausência de sobreposição de processos, preservação dos volumes, HMR no navegador e funcionamento da imagem de produção sem Nuxt. Nenhuma dependência de aplicação, migração ou alteração de CI é necessária.

## Out-of-Scope

Mudanças de domínio, autenticação, rotas REST, eventos, SSR/BFF, separação dos workers, novas réplicas, novos serviços de infraestrutura, refatoração de diretórios e substituição do empacotamento de produção. Não criar um supervisor próprio de build/watch nem um protocolo persistente de arquivos PID; um wrapper de encerramento só entra se a prova com Air nativo falhar. A proposta não autoriza apagar volumes ou dados nem iniciar uma segunda réplica no mesmo banco.
