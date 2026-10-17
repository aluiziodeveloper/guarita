# ADR 0010: Políticas em Rego, com o OPA embutido como biblioteca

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 11; agência excessiva e menor privilégio

## Contexto

Até o Capítulo 10, a política é uma lista de nomes permitidos (`-allow`), igual para todo agente e todo servidor. Um agente que só precisa ler recebe tudo o que outro agente precisou um dia. Uma lista de negação por cima (`examples/denylist`) precisa adivinhar o nome de cada ferramenta perigosa e deixa passar a que tem outro nome.

A Guarita já sabe, desde os Capítulos 8 e 10, quem é o agente e em que servidor a chamada vai rodar. Falta uma política que use isso e que se possa revisar e testar como o resto do código.

## Decisão

1. A porta `PolicyEvaluator` recebe `domain.PolicyInput`: agente, servidor e chamada. A Guarita aplica a decisão (PEP); a política decide (PDP) e não vê o pedido HTTP nem a mensagem MCP.
2. As políticas são escritas em Rego e avaliadas pelo OPA v1.21.1, embutido como biblioteca (`github.com/open-policy-agent/opa/v1/rego`), no processo da Guarita.
3. A consulta é `data.guarita.decision`, preparada uma vez na partida. Uma política que não compila impede a partida; erro de built-in é fatal; resultado indefinido, valor que não é texto ou conflito entre regras viram erro, e o caso de uso nega.
4. A política versionada em `policies/` nega por padrão. As concessões ficam num módulo próprio, `grants.rego`, por agente, servidor e ferramenta, separadas das regras; regras sobre argumentos negam mesmo o concedido.
5. As regras têm testes em Rego (`opa test`) e uma tabela de casos em Go contra os mesmos arquivos. A CI roda `opa fmt`, `opa check --strict` e `opa test` com a versão embutida.
6. `-policy` e `-allow` são exclusivos. `-allow` continua para os laboratórios dos capítulos anteriores.

## Alternativas consideradas

1. **OPA como serviço separado, consultado por HTTP.** Rejeitada: acrescenta um processo, uma rede e um modo de falha (a política fora do ar) a cada chamada, para um único cliente.
2. **Regras em Go, num adaptador próprio.** Rejeitada: a política mudaria com cada compilação, e quem revisa permissões teria de ler Go. Rego separa a regra do código e tem ferramentas de teste e checagem próprias.
3. **Cedar ou outra linguagem de autorização.** Adiada: Rego tem a integração em Go mais madura e um ecossistema de teste estabelecido; a porta permite trocar depois.
4. **Concessões em JSON, como dados do OPA.** Adiada: um módulo Rego com o mapa é igualmente legível, passa pelo mesmo `opa check` e não exige carregar documentos de dados.

## Consequências

- O binário vai de 18.264.523 para 39.078.476 bytes (`CGO_ENABLED=0`, linux/amd64), e o módulo ganha dezenas de dependências indiretas.
- `require_approval` ainda nega: a aprovação humana é o Capítulo 12.
- O agente continua vendo em `tools/list` as ferramentas que não pode chamar; esconder a lista por agente fica para depois.
- No transporte stdio não há agente; só concessões para o agente vazio valem lá.
- Mudar uma permissão é um commit em `policies/`, revisado e testado, e reiniciar a Guarita; não há recarga em execução.
