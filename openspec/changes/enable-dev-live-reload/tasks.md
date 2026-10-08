## 1. Configuração e reverse proxy

- [x] 1.1 Adicionar a configuração opcional do proxy dev e testar variável ausente, URL válida, URL inválida e mensagem sem exposição do valor com `go test ./internal/config -count=1`.
- [x] 1.2 Implementar o proxy e verificar redirecionamento, páginas internas, assets, Host público, query strings, isolamento da API e `503` sem fallback com `go test ./manager ./internal/config ./internal/httpapi/... -count=1`.
- [x] 1.3 Verificar upgrade `101` e comunicação WebSocket nos dois sentidos por um servidor HTTP de teste real com `go test ./manager ./internal/httpapi/... -count=1`.

## 2. Nuxt e HMR

- [ ] 2.1 Ajustar Nuxt e confirmar atualização automática de Vue e CSS por `/manager/` na porta `8081`, sem acesso direto à porta interna, com evidência do navegador.

## 3. Recarga e encerramento do Air

- [x] 3.1 Configurar os recursos nativos do Air e verificar os arquivos observados, compilação automática, erro de build e recuperação após corrigir o código usando Air real.
- [x] 3.2 Testar encerramento de 8 segundos, edições sucessivas e parada do container com processo controlado, exigindo ausência de sobreposição entre execuções Go e registrando o resultado nativo.
- [x] 3.3 Acrescentar proteção mínima somente se a prova nativa falhar e repetir os mesmos cenários até passarem, ou registrar que nenhum wrapper foi necessário se a prova nativa passar integralmente.

## 4. Compose e preservação de dados

- [ ] 4.1 Converter o Compose dev em sobreposição e conferir com `docker compose -f docker-compose.yml -f docker-compose.dev.yml config` que existe apenas um serviço Go e que o frontend não publica porta no host.
- [ ] 4.2 Verificar troca entre compilado e dev, encerramento prévio de eventual Go legado, preservação dos volumes e infraestrutura e atualização de código sem recriar containers, registrando a comparação antes e depois.

## 5. Verificação integrada e documentação

- [ ] 5.1 Executar os checks Go e manager previstos no repositório e validar login, navegação, refresh e reconexão HMR com `agent-browser` em desktop e mobile, registrando resultados e screenshots.
- [ ] 5.2 Confirmar que a imagem de produção serve o manager embutido sem Nuxt em execução e documentar os comandos finais de desenvolvimento, retorno ao compilado e exceções à recarga automática, conferindo-os contra o Compose resolvido.
