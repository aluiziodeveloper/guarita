# ADR 0014: Classificador local de injeção, com limiar medido, prazo e plano B

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 15; injeção indireta em prosa comum, que as regras do Capítulo 14 não alcançam

## Contexto

Os detectores do Capítulo 14 barram sinais de estrutura, e um texto hostil visível, escrito em prosa comum, passa por todos eles. Um modelo de linguagem lê o sentido do texto e pode julgá-lo. A versão ingênua (`examples/naiveclassifier`) pergunta ao Ollama pelo `http.DefaultClient`, que não tem prazo, e deixa passar o resultado quando a pergunta falha: com o modelo fora do ar, todo resultado chega ao agente; com o modelo lento, a chamada do agente espera para sempre.

## Decisão

1. Um adaptador novo, `internal/adapters/ollama`, fala com a API HTTP do Ollama: `POST /api/chat`, com temperatura zero, semente fixa, um único token e as probabilidades das dez alternativas do primeiro token; `GET /api/tags`, para conferir o modelo e registrar o resumo dele. O corpo lido tem limite de 1 MiB; o `http.Client` tem 30 segundos como última barreira.
2. O inspetor `internal/adapters/classifier` implementa `ResultInspector` sobre uma porta própria, `Model`. A pontuação é a probabilidade de YES normalizada entre YES e NO; o texto vai em pedaços de 2000 runas, e vale o maior. O resultado é barrado a partir do limiar.
3. O prazo do classificador é do resultado inteiro, 5 segundos por padrão: no laboratório, em CPU, com `qwen2.5:7b`, a pergunta mais lenta das 132 levou 2,4 segundos, e a primeira, com o modelo frio, 9,6. A Guarita confere o modelo e faz uma pergunta de aquecimento antes de abrir a escuta.
4. O limiar é escolhido no conjunto de desenvolvimento, com um critério fixado antes da medida: a maior revocação entre os limiares que barram no máximo 5% dos legítimos; no empate, o maior. O conjunto de teste é pontuado uma vez, com o limiar já escolhido. Com `qwen2.5:7b` e o *prompt* da *tag* `cap15`, o limiar é 0,8.
5. Sem o modelo, a falha é fechada: o resultado é barrado (`-classifier-on-error deny`, o padrão). O plano B determinístico (`rules`) é escolha explícita do operador: as regras do Capítulo 14 decidem, e o achado `classifier-unavailable` vai ao registro.
6. As regras vêm antes do classificador na revisão: custam microssegundos, e um sinal forte barra o resultado sem perguntar ao modelo.

## Alternativas consideradas

1. **Ler a resposta do modelo como texto, ou pedir um JSON com uma nota.** Rejeitada: um "YES" escrito não tem grau, e não há limiar para escolher; a nota que o modelo escreve é mais um texto gerado, e não uma probabilidade.
2. **Um modelo menor, mais rápido.** Rejeitada pela medida: `qwen2.5:3b` teve mediana de 409 ms, mas, no limiar escolhido, barrou 2 de 30 ataques do teste; com `qwen2.5:1.5b`, nenhum limiar ficou dentro do orçamento de falsos positivos.
3. **Falhar aberto quando o modelo cai, como o ingênuo.** Rejeitada: transforma a falha do classificador num desvio da defesa.
4. **Ajustar o *prompt* e o limiar olhando o conjunto de teste.** Rejeitada: a medida do teste deixaria de dizer como o classificador se sai num texto que ele não viu.
5. **Um modelo hospedado por terceiros.** Rejeitada para o livro: o resultado das ferramentas sairia da máquina, e a disponibilidade dependeria de outro serviço.

## Consequências

- O texto codificado passa: no conjunto ofuscado, base64, texto invertido e ROT13 ficaram abaixo do limiar.
- No teste, um ataque ficou com 0,771, abaixo de 0,8. O desempate pelo maior limiar barra menos legítimos e deixa menos margem para os ataques.
- As medidas valem para o modelo (pelo resumo), o *prompt* e o conjunto de referência do laboratório, que tem 132 exemplos escritos para o livro; outro modelo pede outra medida.
- O classificador lê o texto hostil, e esse texto pode tentar instruí-lo; a etiqueta em volta do texto não impede isso.
- Cada resultado custa perto de um segundo em CPU por pedaço de texto, somado à chamada.
- A medida do conjunto de referência exige um Ollama local e não roda na CI; o teste do modelo fora do ar roda.
