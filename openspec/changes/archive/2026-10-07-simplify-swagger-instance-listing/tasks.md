## 1. Backend

- [x] 1.1 Retirar paginação do HTTP, serviço e repositório de instâncias e adaptar stats, restauração e scheduler, verificando testes de listagem, escopo e Postgres com mais de 100 itens.

## 2. Manager

- [x] 2.1 Atualizar tipos e consumidores da coleção para uma chamada sem cursor, preservando paginação local, verificando typecheck e build Nuxt.

## 3. Swagger

- [x] 3.1 Remover parâmetros manuais apikey e X-Request-Id de todas as operações, gerar documentos e registrar o contrato no README, verificando os testes Swagger e a geração sem diff.

## 4. Revisão e integração

- [x] 4.1 Revisar a alteração e executar gofmt, vet, lint, suíte Go completa e build, registrando verify.md e retrospective.md com evidências.
- [x] 4.2 Integrar a alteração na main preservando as mudanças locais e atualizar o serviço local, verificando no navegador Authorize uma vez e GET /instances sem os quatro campos removidos.
