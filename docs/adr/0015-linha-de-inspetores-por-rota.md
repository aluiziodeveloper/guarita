# ADR 0015: Linha de inspetores por rota, montada por registro, com camada exigida

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 16; composição dos inspetores dos Capítulos 13 a 15 e defesa em profundidade

## Contexto

Até o Capítulo 15, as opções dos inspetores valiam para todas as rotas, e a ordem entre eles estava escrita à mão em duas funções de `cmd/guarita`. O caso de uso repetia o mesmo laço na ida e na volta, com tipos de inspetor diferentes. Ligar o classificador numa rota só, para economizar latência, exigia ligá-lo em todas. A cena do capítulo mostra o risco de dar a cada rota só o inspetor que parece ser dela: uma página com texto em base64 passa pelas regras da volta, que não a leem, e o envio pela rota de e-mail, sem nenhuma camada, deixa o CPF sair. A camada que pararia o ataque existia, mas em outra rota.

## Decisão

1. O caso de uso passa a ida e a volta pelo mesmo laço genérico, `runStages[I]`, em que `I` é `Inspector` ou `ResultInspector`. O laço devolve, como ponteiro, a inspeção que parou a fila; a primeira versão a devolvia por valor, e a inspeção zero, com decisão vazia, deixava passar um resultado com decisão vazia, que antes era barrado.
2. O pacote `internal/pipeline` monta a linha de uma rota a partir de nomes, na ordem escrita, com um registro de fábricas. Um nome fora do registro, repetido, um marcador antes do fim ou uma etapa sem inspetor recusam a rota na partida.
3. As cinco etapas (`schema`, `dlp`, `injection`, `classifier`, `mark`) ficam num registro em `cmd/guarita`. Uma etapa nova é uma entrada nova; a montagem, o caso de uso e o transporte não mudam. As opções dos capítulos anteriores viram a linha padrão, na ordem que sempre tiveram.
4. `-pipeline nome=etapa,…` dá a linha de uma rota de `-server`; sem nome, vale para as rotas sem linha própria. Um nome de rota que não existe é erro.
5. `-require-stages` recusa a partida de uma rota sem as etapas exigidas.
6. O teste de camada ausente, em `attacks/pipeline`, tira uma camada de cada vez: sem cada uma, passam exatamente os ataques dela, e os legítimos passam sempre.

## Alternativas consideradas

1. **Um arquivo de configuração em YAML ou JSON para as linhas.** Adiada: as linhas cabem em opções de linha de comando, e um formato novo de arquivo pede validação própria e uma dependência. A questão volta na operação, no Capítulo 21, se as linhas crescerem.
2. **Ordenar as etapas automaticamente, por custo.** Rejeitada: a ordem tem efeito sobre a segurança, e não só sobre o custo (a marcação precisa ser a última; uma regra barata antes do modelo evita a pergunta), e a ordem escrita é a ordem que o operador revisa.
3. **Rodar as etapas em paralelo.** Rejeitada por enquanto: a fila para no primeiro bloqueio, e o classificador, que domina a latência, é justamente o que a parada evita chamar. Em paralelo, ele seria chamado mesmo quando uma regra de microssegundos já tivesse barrado o resultado.
4. **Uma interface única, com um método genérico.** Rejeitada: Go não tem métodos com parâmetros de tipo, e as duas portas continuam com nomes e argumentos próprios; o genérico fica no laço que as consome.

## Consequências

- No *benchmark*, com 1 KiB de texto legítimo, a mediana de cinco execuções por ida e volta foi de 2,3 µs sem etapas, 8,8 µs com o esquema, 127,9 µs com o DLP, 68,0 µs com as regras de injeção e 11,4 µs com a marcação; as linhas completas das duas rotas, 135,6 µs e 78,9 µs. O classificador do Capítulo 15, com mediana de 853 ms por pergunta em CPU, custa quatro ordens de grandeza a mais que qualquer etapa determinística.
- Uma rota com linha própria sem `dlp` não inspeciona os argumentos, a menos que `-require-stages` a recuse.
- O transporte stdio continua sem inspetores.
- O teste de camada ausente cobre as etapas determinísticas; o classificador entra só nas medidas com Ollama.
