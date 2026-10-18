# ADR 0012: Inspeção determinística dos argumentos antes de qualquer modelo

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 13; exfiltração de dados nos argumentos

## Contexto

A política decide quem chama o quê, e a aprovação põe uma pessoa diante das ações perigosas. Nenhuma das duas olha o conteúdo de uma chamada permitida: um CPF, uma chave de API ou um destino fora da organização saem nos argumentos de uma ferramenta concedida. A versão ingênua (`examples/naivedlp`) procura o CPF formatado e nomes de campo como `password`, deixa passar o CPF sem pontuação, a chave num campo qualquer e o dado em base64, e acusa números de pedido com o formato do CPF.

## Decisão

1. Uma porta nova, `Inspector`, examina a chamada que a política não negou. Os inspetores rodam em ordem, e a decisão mais severa vence (`domain.Stricter`); uma decisão fora do conjunto conta como negação, e um inspetor que falha nega.
2. Os primeiros inspetores são determinísticos: padrões, validadores, medidas e listas. Os classificadores com modelo de linguagem dos Capítulos 14 e 15 entram depois deles, como outras implementações da mesma porta.
3. O inspetor DLP percorre todas as strings dos argumentos e acha CPF com os dígitos verificadores, formatos públicos de chave, o mesmo dentro de base64 decodificado e tokens de entropia alta. A decisão por sinal fica numa tabela: chave nega; CPF e entropia pedem aprovação.
4. Com `-allow-hosts`, toda URL nos argumentos precisa apontar para um host da lista, extraído com `url.Parse`.
5. Com `-validate-arguments`, os argumentos são validados contra o `inputSchema` da definição fixada, com a biblioteca que já vinha com o SDK do MCP, sem `Loader` para `$ref` remoto.
6. Um achado nunca carrega o valor: só o campo, o sinal e um detalhe sem o dado.
7. Os detectores têm testes de propriedade com semente fixa, e a taxa de falsos positivos é medida num corpus benigno gerado.

## Alternativas consideradas

1. **Classificar os argumentos com um modelo de linguagem desde já.** Adiada: um modelo erra de modos que não se medem com uma tabela, custa latência a cada chamada e pode ser enganado pelo mesmo texto que engana o agente. Regras determinísticas primeiro, modelo depois, para o que elas não alcançam.
2. **Reescrever os argumentos, mascarando o dado.** Rejeitada: a mesma objeção do Capítulo 10 à normalização; a chamada que chega ao servidor deixa de ser a que o agente pediu e a que a política avaliou.
3. **Ligar o DLP por padrão.** Adiada: os falsos positivos medidos (blobs em base64 pedem aprovação) mudariam o comportamento de todo laboratório anterior; o operador liga com `-dlp`.
4. **Uma biblioteca nova de JSON Schema.** Rejeitada: `github.com/google/jsonschema-go` já está no módulo pelo SDK.

## Consequências

- 6 em 1000 números de 11 dígitos passam como CPF e pedem aprovação; 1000 em 1000 blobs aleatórios em base64 de 32 bytes também.
- Segredos em hexadecimal ficam abaixo do limiar de entropia e passam, a menos que tenham um prefixo conhecido.
- Dado cifrado, comprimido ou partido em várias chamadas passa por todos os detectores.
- O transporte stdio ainda não tem inspetores.
- As respostas dos servidores não são inspecionadas: é o Capítulo 14.
