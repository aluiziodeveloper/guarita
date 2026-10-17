# ADR 0009: Varredor estrutural antes do pin e um namespace por servidor

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 10; envenenamento e sombreamento de ferramentas

## Contexto

O catálogo do Capítulo 9 fixa a primeira definição sem ler o que ela diz. Uma descrição pode trazer texto entre caracteres que o operador não vê e o modelo lê. E, com vários servidores atrás da mesma Guarita, um deles pode anunciar uma ferramenta com o nome de outra, para o modelo escolher pela descrição.

Duas defesas ingênuas falham (`examples/naivescan`): uma lista de palavras não acha o texto que não as usa; reescrever o texto com NFKC entrega ao agente uma definição diferente da que o servidor mandou e da que o operador revisou.

## Decisão

1. Antes do *pin*, a definição inteira (chaves e valores, em qualquer profundidade) passa por um varredor estrutural, atrás da porta `DefinitionScanner`. Os sinais são objetivos: caractere de formatação (categoria Cf), de controle (Cc, exceto `\n`, `\r` e `\t`), texto em que NFC e NFKC diferem, nome fora de `[A-Za-z0-9_.-]{1,128}`.
2. Validar, não normalizar: a definição com sinal é recusada inteira, nunca reescrita. A Guarita repassa só os bytes que fixou.
3. A recusa não fixa nada: o servidor que corrigir a definição recebe confiança no primeiro uso depois de corrigi-la. Cada recusa vai ao registro com servidor, ferramenta, motivo e sinais, e uma chave não ASCII aparece com escapes.
4. Cada servidor recebe do operador um namespace e fica em `/mcp/{namespace}`. O nome não vem de `serverInfo`, que o próprio servidor escolhe. Cada caminho é um recurso protegido próprio, com audiência, metadados e credencial próprios.
5. Um catálogo só para todos os servidores, com a silhueta do nome (minúsculas ASCII, `-` e `.` como `_`) guardada pela migração 0003. A ferramenta nova cuja silhueta outro servidor já fixou é recusada: o primeiro fica com o nome.

## Alternativas consideradas

1. **Normalizar com NFKC e seguir.** Rejeitada: cria duas leituras da mesma definição e esconde do operador o que o servidor mandou.
2. **Lista de palavras ou expressões regulares sobre o sentido.** Rejeitada como defesa; o sentido é dos classificadores dos Capítulos 14 e 15.
3. **Remover os caracteres invisíveis e repassar o resto.** Rejeitada: é uma normalização, com o mesmo problema da primeira.
4. **Prefixar o nome de cada ferramenta com o namespace (`files__read_file`).** Adiada: muda o nome que o servidor espera receber e exige reescrever `tools/call`. O caminho por servidor já separa as sessões.
5. **Recusar todas as marcas combinantes (Mn).** Rejeitada: recusaria acentos decompostos legítimos. O desafio do capítulo fecha só as variantes sem uso em texto.

## Consequências

- Texto hostil visível, em letras comuns, passa pelo varredor; ele só garante que o operador vê o que o modelo lê.
- Uma descrição legítima com caractere de formatação (um hífen suave, um *joiner* num emoji) é recusada; o operador vê o motivo no registro. É um falso positivo aceito.
- Seletores de variação e o *combining grapheme joiner* passam até o desafio do Capítulo 10.
- Quem anuncia primeiro fica com o nome, ainda que seja o servidor hostil; a ordem de partida passa a ser decisão do operador.
- `-upstream` continua em `/mcp`, com a URL como chave no catálogo, compatível com os bancos do Capítulo 9.
