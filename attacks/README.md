# Ataques de laboratório

Cada pacote deste diretório reproduz um ataque contra a Guarita, com um servidor MCP de laboratório e um teste que roda o ataque duas vezes: sem a defesa, ele precisa funcionar; com a defesa, precisa falhar. Um ataque que volte a funcionar quebra o *build*.

Os servidores daqui existem só para o laboratório do livro. Eles rodam dentro dos testes, em `127.0.0.1`, usam marcadores `[LAB]` no lugar de conteúdo nocivo e não devem ser usados fora deste repositório.

| Pacote | Ataque | Defesa | Capítulo |
|---|---|---|---|
| `rugpull` | O servidor troca o esquema de uma ferramenta depois da primeira chamada | Catálogo com *pin* pela impressão digital | 9 |
