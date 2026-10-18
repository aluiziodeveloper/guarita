# ADR 0017: Cotas por balde de fichas, orçamento de custo e disjuntor no repasse

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 18; abuso de recursos por laço de chamadas e servidores que falham devagar

## Contexto

Um agente em laço chama a mesma ferramenta paga até alguém perceber a fatura; no laboratório, 200 perguntas de 30 unidades chegaram todas ao servidor, 6000 unidades. A cota ingênua, um contador por minuto do relógio (`examples/naivequota`), deixou passar 120 chamadas em três segundos com um limite de 60 por minuto, 60 de cada lado da virada. E um servidor que demora para falhar segura cada chamada do agente: 20 chamadas a um servidor que responde 502 depois de 200 ms levaram cerca de 4 segundos.

## Decisão

1. A cota é um balde de fichas pelo GCRA: um só `atomic.Int64` guarda o instante teórico em que o balde estaria cheio, atualizado por compare-and-swap, com o tempo pelo relógio monotônico. Com 60 por minuto e rajada de 60, as mesmas 120 chamadas em três segundos deixaram passar 62.
2. Há um balde por agente, servidor e ferramenta (`-quota`), e um orçamento de unidades por agente (`-budget`), de onde cada chamada tira o custo estimado da ferramenta (`-cost`). O orçamento é do agente em todas as rotas, e o inspetor é um só para a Guarita inteira. Os baldes ficam num `sync.Map`, com chave de estrutura.
3. A cota é a etapa `quota` da linha de inspetores do Capítulo 16, a primeira da ida na linha padrão: um laço barrado não paga as outras etapas. A recusa é uma negação da política, num erro JSON-RPC, e o achado diz quanto esperar.
4. O disjuntor envolve o `http.RoundTripper` do repasse de cada servidor (`-breaker-failures`, `-breaker-cooldown`): abre depois de falhas seguidas, erro de rede ou status 5xx, recusa na hora com 503 e `Retry-After`, e depois do resfriamento deixa passar uma única chamada de teste. Os estados seguem uma tabela, como as aprovações do Capítulo 12. O transporte lê o tempo de espera por uma interface própria, e o disjuntor a satisfaz sem que um pacote importe o outro.

## Alternativas consideradas

1. **`golang.org/x/time/rate`.** Rejeitada para o livro: resolve o mesmo problema, com uma trava, e esconderia o algoritmo que o capítulo ensina. Uma instalação que prefira a dependência troca a implementação do balde sem mudar o inspetor.
2. **Janela fixa ou janela deslizante com registro de cada chamada.** Rejeitada: a fixa deixa passar o dobro na virada, como a medida mostrou; a deslizante guarda um registro por chamada, e a memória cresce com o laço que ela devia conter.
3. **O custo real devolvido pelo servidor.** Adiada: o MCP não tem um campo padrão de custo no resultado, e a cota precisa decidir antes da chamada.
4. **Cotas compartilhadas entre réplicas, num armazenamento externo.** Adiada para a operação, no Capítulo 21; a Guarita do livro é uma instância só.

## Consequências

- O compare-and-swap não é sempre mais rápido. Num só balde disputado por todas as *goroutines*, ele ganhou com 1 CPU, empatou com 4 e perdeu com 20, para a mesma conta com `sync.Mutex`: sob disputa, as tentativas repetidas custam mais que a espera da trava (medianas em `s18-benchmark-cpu.txt`, no repositório do livro). A disputa por um balde só é a de um agente batendo numa ferramenta só; no caso comum, com um balde por agente e ferramenta, a etapa inteira levou cerca de 16 ns por chamada com 20 CPUs. A decisão fica com o balde sem trava, que é o algoritmo que o capítulo ensina, e o custo medido nos dois casos, desprezível perto de uma chamada de rede. A primeira versão do *benchmark* usava uma taxa cujo intervalo, pela divisão inteira, era zero, e não tirava fichas de verdade.
- Os baldes vivem em memória e recomeçam cheios quando a Guarita reinicia.
- Uma chamada barrada pelo orçamento já tirou a ficha da cota de chamadas.
- O custo é uma estimativa por ferramenta.
- O disjuntor conta como sucesso um erro de ferramenta devolvido com status 200.
- O transporte stdio não tem cotas.
