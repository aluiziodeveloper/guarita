# ADR 0007: Identidade do agente e credencial própria da Guarita

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 8; estende o ADR 0006 e adota a revisão `2026-07-28` do MCP

## Contexto

No Capítulo 7, a Guarita repassava ao servidor todos os cabeçalhos do agente, inclusive `Authorization`. Um servidor de terceiros recebia um *token* válido do usuário e podia usá-lo em qualquer serviço que o aceitasse. A especificação do MCP chama isso de *token passthrough* e o proíbe: um servidor só aceita *tokens* emitidos para ele e não repassa nenhum outro.

Duas correções ingênuas falham: verificar a assinatura do JWT sem conferir a audiência aceita um *token* do mesmo emissor pedido para outro recurso (`examples/noaudience`); apagar só o `Authorization` deixa o *token* seguir em `Cookie` e `X-Api-Key` (`examples/stripauth`).

A revisão `2026-07-28` do MCP passou a espelhar `method` e `params.name` em cabeçalhos, para que intermediários roteiem sem ler o corpo, e exige que quem lê o corpo recuse a divergência.

## Decisão

1. O agente se autentica com JWT assinado em Ed25519. A Guarita aceita só o algoritmo `EdDSA`, o emissor configurado, a audiência igual ao recurso que ela protege, `exp` e `iat` presentes, 30 s de tolerância de relógio e sujeito não vazio.
2. A verificação usa `github.com/golang-jwt/jwt/v5`, a mesma versão de que o SDK depende, e o middleware `auth.RequireBearerToken` do SDK: 401 com `WWW-Authenticate` apontando os metadados do recurso (RFC 9728), servidos em `/.well-known/oauth-protected-resource`.
3. Só uma lista de cabeçalhos chega ao servidor: `Accept`, `Content-Type`, os do MCP nas revisões `2025-11-25` e `2026-07-28` e `Mcp-Param-*`. Uma lista de bloqueio deixaria passar o próximo cabeçalho que alguém inventasse.
4. A Guarita se apresenta ao servidor com a própria credencial, lida de um arquivo com permissão `0600`.
5. Segredos têm tipo próprio (`secret.Secret`), que se redige em `fmt`, `slog` e JSON; o valor só sai por `Reveal`.
6. O registro de acesso usa `log/slog` com campos escolhidos um a um; nenhum cabeçalho nem corpo é registrado.
7. Sem `-agent-key`, a Guarita só escuta em loopback; com `-host`, a partida falha.
8. Na revisão `2026-07-28`, `MCP-Protocol-Version`, `Mcp-Method` e `Mcp-Name` precisam vir uma vez só e iguais ao corpo; a divergência recebe 400 com o código `-32020`. `subscriptions/listen` não tem prazo de chamada.

## Alternativas consideradas

1. **Repassar o *token* do usuário para servidores "confiáveis".** Rejeitada: é o *token passthrough* que a especificação proíbe, e a confiança no servidor não limita onde o *token* vale.
2. **Troca de *token* (RFC 8693) para emitir uma credencial por pedido.** Adiada: exige um servidor de autorização que o livro não constrói; a credencial estática por servidor cobre o laboratório.
3. **RS256 ou HS256.** Ed25519 tem chaves curtas, assinatura determinística e um único algoritmo a aceitar; HS256 exigiria compartilhar o segredo de assinatura com a Guarita.
4. **Escrever o middleware de *bearer*.** Rejeitada: o do SDK já segue a RFC 6750 e a RFC 9728, e o texto explica o que ele faz.

## Consequências

- O operador precisa da chave pública do emissor e de um arquivo de credencial com permissão `0600`.
- A audiência é o endereço exato do recurso; atrás de outro nome, ele vai em `-resource`.
- Uma credencial só, para um único servidor. Vários servidores, cada um com a sua, chegam no Capítulo 10; os escopos por ferramenta e agente, na política do Capítulo 11.
- O transporte stdio continua sem identidade do agente: quem inicia o processo é o próprio agente.
- Trocar a chave do emissor exige reiniciar a Guarita; aceitar mais de uma chave pelo `kid` é o desafio do Capítulo 8.
