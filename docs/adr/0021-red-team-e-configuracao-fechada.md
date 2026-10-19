# ADR 0021: Red team como regressão integrada e configuração em falha fechada

- **Estado:** aceita
- **Data:** 2026-10-19
- **Contexto:** Capítulo 22; a revisão de segurança antes da 1.0.0

## Contexto

Cada capítulo provou uma defesa contra o próprio ataque, no pacote dela. A soma desses testes não dizia nada sobre a Guarita com tudo ligado, nem sobre o que acontece entre duas defesas. O *red team* do Capítulo 22 combinou os laboratórios contra uma só Guarita e achou duas lacunas: no modo stdio, opções de defesa eram aceitas e ignoradas; e a revisão das respostas deixava passar o erro JSON-RPC.

## Decisão

1. **`attacks/redteam` é regressão integrada.** A suíte compila a Guarita como a *release* e roda a cadeia de movimentos na CI, no job dos ataques. Uma defesa que deixe de valer junto das outras quebra o *build*, mesmo que o teste do pacote dela continue verde.
2. **Configuração em falha fechada.** Uma opção que o modo escolhido não aplica impede a partida. O modo stdio tem uma lista de permissão das opções que aplica; uma opção nova fica de fora até alguém a ligar ali, e a etapa de volta, que o stdio não sabe executar, também impede a partida.
3. **A linha de ida nos dois modos.** O stdio monta a mesma linha das rotas HTTP, com DLP, destinos e cota, pelo mesmo registro.
4. **O erro revisado como o resultado.** O objeto de erro da resposta de `tools/call` passa pelos inspetores da volta; barrado, mantém o código e leva o aviso da Guarita.
5. **Risco residual declarado.** O que o *red team* não fechou vai para o relatório de segurança, com o caminho no *roadmap*, e nenhuma ameaça passa a `mitigated`.

## Alternativas consideradas

1. **Avisar no registro, em vez de recusar a partida.** Rejeitada: é a falha do Capítulo 21 de novo, o aviso que ninguém lê.
2. **Uma lista de bloqueio das opções do HTTP.** Rejeitada: a opção nova de um capítulo futuro seria aceita em silêncio no stdio, que é exatamente a lacuna achada.
3. **Revisar as respostas também no stdio neste capítulo.** Adiada (D32): exige abrir as mensagens no `mcpproxy.Relay`, uma mudança de outro tamanho.

## Consequências

- O modo stdio fica mais estrito: quem passava `-inspect-results` ou `-ops-listen` no stdio, sem efeito, agora vê a partida falhar com o nome da opção.
- Uma chamada com CPF no stdio é negada, e não enfileirada: o stdio não tem fila de aprovação.
- A suíte leva cerca de 4 segundos no laboratório, a maior parte esperando o prazo de 2 segundos da aprovação.
- A *tag* `cap22` não é a 1.0.0: ela sai no dia da edição 1.0 do livro (D30).
