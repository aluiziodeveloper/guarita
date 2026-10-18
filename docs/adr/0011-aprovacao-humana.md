# ADR 0011: Aprovação humana com fila em memória e interface do operador separada

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 12; autonomia excessiva, fadiga de aprovação e aprovação forjada

## Contexto

Desde o Capítulo 11, a política responde `require_approval` para chamadas de alto impacto, e a Guarita trata a resposta como negação. Falta uma pessoa capaz de dizer sim, como a especificação do MCP recomenda. Uma aprovação mal feita, porém, é pior que nenhuma: na versão ingênua (`examples/naiveapproval`), o agente aprova o próprio pedido por um endereço do mesmo servidor, com números sequenciais, e uma fila sem limite convida o operador a ceder pela insistência.

## Decisão

1. Os estados da aprovação ficam no domínio, numa tabela de transições: `pending` vai para `approved`, `denied`, `expired` ou `canceled`, e os quatro são terminais. Toda mudança passa pela tabela.
2. A porta `Approver` faz `require_approval` esperar. Sem aprovador, nada muda: a chamada é negada.
3. A fila fica em memória (`approval.Queue`). Cada pedido recebe um identificador de `rand.Text`; a espera é um `select` entre o canal fechado pela decisão, o prazo e o contexto do agente.
4. Contra a fadiga: no máximo três pedidos pendentes por agente e a mesma chamada negada recusada na hora por dez minutos, com os argumentos na forma canônica.
5. A interface do operador fica noutra escuta, só em loopback, atrás da guarda de Host, de `http.CrossOriginProtection` e de uma credencial própria, lida de um arquivo com permissões restritas e comparada em tempo constante. O token do agente não vale ali, e o agente não decide o próprio pedido.
6. O operador decide pela API, por uma página em `html/template` ou pelo subcomando `guarita approvals`.

## Alternativas consideradas

1. **Aprovação pelo próprio protocolo (elicitação do MCP).** Adiada: a elicitação pede algo ao usuário do cliente, que é o lado do agente; a aprovação precisa vir de fora dele.
2. **Fila em banco ou serviço de mensagens.** Adiada: uma chamada que espera aprovação morre com a conexão do agente; persistir o pedido só faz sentido junto com a auditoria do Capítulo 19.
3. **Tokens de operador do mesmo emissor dos agentes.** Rejeitada para agora: um emissor comprometido emitiria as duas coisas. Credenciais separadas mantêm os dois planos separados.
4. **Aprovação por padrão de chamada (“aprovar sempre `delete_database`”).** Rejeitada: transforma uma decisão pontual numa concessão, que é papel da política versionada.

## Consequências

- Uma chamada que espera aprovação segura a conexão do agente até a decisão ou o prazo; o cliente do agente precisa tolerar a espera.
- Reiniciar a Guarita perde os pedidos pendentes; as chamadas falham, e o agente pede de novo.
- O agente pode variar os argumentos para escapar da recusa da repetição; o limite de pendentes continua valendo.
- A aprovação não tem registro durável de quem decidiu o quê: é o Capítulo 19.
- Os segredos dos operadores são estáticos; MFA e expiração ficam fora do escopo do laboratório.
