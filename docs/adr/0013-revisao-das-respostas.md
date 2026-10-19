# ADR 0013: Revisão do resultado das ferramentas e marcação do conteúdo não confiável

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 14; injeção indireta de prompt nas respostas

## Contexto

Os Capítulos 9 a 13 examinam o que o servidor anuncia e o que o agente manda. O resultado de cada chamada chega ao modelo sem ser lido, e uma página, um arquivo ou a resposta de uma API pode trazer texto escrito para o modelo. A versão ingênua (`examples/naiveinjection`) procura frases conhecidas, deixa passar a mesma instrução em outras palavras, partida por um caractere invisível ou escrita em *tags* Unicode, e barra um artigo que só cita a frase.

## Decisão

1. O transporte HTTP passa o resultado de cada `tools/call`, em JSON e em SSE, pelo caso de uso (`ReviewResult`) antes de entregá-lo ao agente. A reescrita de respostas do Capítulo 9 foi generalizada para isso, sem mudar o comportamento dela.
2. Duas portas novas: `ResultInspector`, que examina o resultado, e `ResultMarker`, que marca o que passou.
3. No caminho de volta não há aprovação humana: um inspetor que não permite barra o resultado inteiro, que chega ao agente como erro de ferramenta (`isError`) com o motivo, e a sessão continua.
4. O inspetor de injeção separa sinais fortes, que barram (caracteres invisíveis e de controle, delimitadores de modelos de conversa, estilos que escondem texto), de sinais fracos, que só vão ao registro (rótulos de papel, comentários HTML). A separação veio da medida: a primeira versão, que barrava tudo, barrou 5 de 12 resultados legítimos do corpus.
5. O marcador envolve o texto de cada resultado entre delimitadores com o servidor, a ferramenta e um identificador de `rand.Text` sorteado a cada resposta, para que o conteúdo não consiga imitar o fim do bloco.
6. As duas coisas ligam por opção: `-inspect-results` e `-mark-untrusted`.

## Alternativas consideradas

1. **Remover do resultado o trecho suspeito.** Rejeitada: a mesma objeção dos Capítulos 10 e 13 à reescrita; o agente receberia um resultado que o servidor não mandou, sem saber.
2. **Classificar o resultado com um modelo de linguagem.** Adiada para o Capítulo 15, como outra implementação de `ResultInspector`, depois das regras determinísticas.
3. **Barrar também os sinais fracos.** Rejeitada pela medida: um terço das respostas legítimas do corpus seria barrado.
4. **Marcar com um delimitador fixo.** Rejeitada: um conteúdo hostil escreveria o delimitador de fim e sairia do bloco.

## Consequências

- Um texto hostil visível, em prosa comum, passa por todos os detectores; os sinais fracos chegam ao agente marcados.
- A marcação só ajuda se o modelo do agente for instruído a tratar o bloco marcado como dado, e não como instrução; a Guarita não controla esse *prompt* e, neste capítulo, não mede o efeito.
- Blocos que não são texto, `structuredContent` e recursos embutidos não são marcados.
- O transporte stdio não revisa respostas.
- Cada resposta de `tools/call` é lida e reescrita, com o custo de decodificar e codificar o JSON dela.

## Revisão do Capítulo 22

O *red team* pôs a página de ataque na mensagem do erro JSON-RPC, que a revisão deixava passar. Desde a ADR 0021, o erro passa pelos mesmos inspetores; a marcação continua só nos blocos de texto. O stdio continua sem revisar respostas, e agora recusa as opções da volta.
