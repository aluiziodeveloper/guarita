# Relatório de segurança da Guarita

- **Escopo:** a Guarita na *tag* `cap22`, nos dois modos (HTTP com rotas por servidor e stdio com um servidor local), com as opções de defesa dos Capítulos 7 a 21.
- **Data:** 19/10/2026.
- **Método:** *red team* de laboratório: os ataques dos capítulos anteriores, sem técnica nova, combinados contra uma só Guarita compilada como a *release*. A suíte é `attacks/redteam`, e toda afirmação abaixo tem um teste ou uma leitura de código indicada. O guia técnico do NIST para testes de segurança [1] orientou a separação entre o que foi testado, o que foi revisado no código e o que fica fora.
- **Ambiente:** go1.27.2, SDK MCP v1.8.0, OPA v1.21.2, Linux (WSL 2). Nenhum sistema de terceiros foi alvo; os servidores de ataque rodam dentro dos testes, em `127.0.0.1`, com marcadores `[LAB]`.

## 1. Resultado

Os treze movimentos da cadeia falharam contra a Guarita no modo HTTP. A primeira rodada achou duas lacunas, as duas fechadas no Capítulo 22. Nenhuma ameaça do modelo está `mitigated`: todas estão `partially-mitigated`, com as lacunas declaradas em [`threats/model.md`](threats/model.md) e o risco residual na seção 4.

Este relatório é evidência, e não garantia. Ele diz que os ataques do laboratório falham contra esta versão, com estas opções; não diz que ataques que o livro não escreveu falham.

## 2. Os movimentos da cadeia

| # | Movimento | Defesa que respondeu | Resultado |
|---:|---|---|---|
| 1 | Ferramenta com caractere de largura zero na descrição | Varredor do catálogo (Cap. 10) | Fora de `tools/list` |
| 2 | Segundo servidor anuncia `Read-File` | Silhueta do nome (Cap. 10) | Fora da lista; chamada negada |
| 3 | O servidor troca o esquema depois da primeira chamada | *Pin* pela impressão digital (Cap. 9) | Chamada negada |
| 4 | Onze páginas de ataque na busca | Revisão e marcação (Cap. 14) | 7 barradas, 4 marcadas |
| 5 | CPF e destino de fora no envio | DLP e destinos (Cap. 13) | CPF na fila, expira; destino negado; só o legítimo chega |
| 6 | `delete_database` para o agente de relatórios | Política em Rego (Cap. 11) | Negada |
| 7 | Cinco pedidos de aprovação seguidos | Limite de pendentes e recusa da repetição (Cap. 12) | 3 na fila, 2 negados; a repetição negada |
| 8 | Laço de 30 buscas | Cota de 20 por minuto (Cap. 18) | 9 passaram, 21 negadas (as 11 da injeção já pagaram a cota) |
| 9 | Token de uma rota usado em outra | Audiência do token (Cap. 8) | 401 |
| 10 | Negação trocada por permissão na trilha | Cadeia de *hashes* (Cap. 19) | O verificador acusa a linha 1 |
| 11 | `/metrics` e `/debug/pprof/` pela operação | Escuta de operação (Cap. 21) | Métricas sem dados; perfis 404 |
| 12 | Página de ataque no erro JSON-RPC | Revisão do erro (Cap. 22) | Barrada |
| 13 | Host de fora; chave `id` repetida | Guarda de Host (Cap. 7); *parser* estrito (Cap. 6) | 403; 400 |

A cadeia de suprimentos (Cap. 20) fica com `attacks/supplychain`, porque o digest do executável só existe no modo stdio.

## 3. Achados

| Achado | Evidência | Correção | Estado |
|---|---|---|---|
| No modo stdio, `-dlp`, `-quota`, `-require-stages` e as opções da volta eram aceitas e ignoradas; o CPF chegava ao servidor e o laço não achava a cota | `TestCostura_stdio` e `TestCostura_stdioRecusa` no commit `3b3abd0` | O stdio aplica a linha de ida e recusa na partida, por lista de permissão, o que não aplica (commit `4bcf5b1`) | Fechado |
| A revisão das respostas deixava passar o erro JSON-RPC; a página de ataque chegava inteira na mensagem | Movimento 12 no commit `4bcf5b1` | O erro passa pelos inspetores; barrado, leva o aviso da Guarita (commit `4381470`) | Fechado |

O primeiro achado não estava registrado em nenhum documento: cada ADR dizia que o stdio não tinha aquela defesa, mas nenhuma dizia que a opção seria aceita. O segundo estava no modelo de ameaças desde o Capítulo 14, como lacuna; o *red team* mostrou que o próprio corpus do livro a explorava.

## 4. Risco residual

| Risco | Por que fica | Caminho |
|---|---|---|
| O stdio não revisa respostas, e só o stdio tem sandbox: isolamento e revisão não se combinam numa implantação | O repasse do stdio copia as mensagens do servidor sem abri-las (`mcpproxy.Relay`) | *Roadmap*, item 1 e 2 |
| Pedidos que não são `tools/call` (`resources/read`, `prompts/get`) seguem sem política, e as respostas deles, sem revisão | A porta só decide chamadas de ferramenta (`mcpproxy.Gate.Admit`) | *Roadmap*, item 3 |
| Notificações no fluxo da resposta chegam sem revisão | A revisão olha só a resposta (`mcphttp.reviewMessage`) | Desafio do Capítulo 22; *roadmap*, item 4 |
| `structuredContent`, recursos embutidos e o erro JSON-RPC não são marcados; sinais fracos neles passam sem delimitador | A marcação só embrulha blocos de texto (`untrusted.Marker.Mark`) | *Roadmap*, item 5 |
| A marcação não reduziu a obediência do modelo no laboratório | Medida no Capítulo 15: 6 de 30 sem marca, 9 a 12 com ela | Fora do alcance de um proxy; o *prompt* do agente decide |
| Texto codificado passa pelo classificador; dado cifrado, comprimido ou partido em várias chamadas passa pelo DLP | Limites medidos nos Capítulos 13 e 15 | *Roadmap*, item 6 |
| Cotas em memória, por processo | Reiniciar enche os baldes; duas Guaritas não dividem cotas | *Roadmap*, item 7 |
| A confiança no primeiro uso fixa a primeira definição que chega | *Pin* sem aprovação prévia (Cap. 9) | Aprovação do catálogo pelo operador, *roadmap*, item 8 |
| Sem reprodução em máquina física distinta nem revisão técnica externa | Dispensada pelo autor para avançar (D09) | Fase 5 do livro |
| A *release* foi assinada só no laboratório, sem registro público de transparência | O laboratório não envia nada ao Rekor (Cap. 20) | `v1.0.0` no dia da edição 1.0 |

## 5. Mapa para o OWASP Top 10 para LLMs

| Risco OWASP 2025 | Ameaças da Guarita | Defesa principal |
|---|---|---|
| LLM01 Prompt Injection [2] | `indirect-prompt-injection`, `tool-poisoning` | Revisão e marcação das respostas; varredor do catálogo |
| LLM02 Sensitive Information Disclosure [3] | `data-exfiltration`, `audit-tampering`, `ops-exposure` | DLP e destinos; HMAC dos argumentos na trilha; operação sem dados |
| LLM03 Supply Chain [4] | `supply-chain`, `rug-pull` | Digest do executável; *pin* das definições |
| LLM05 Improper Output Handling [5] | `indirect-prompt-injection` | O resultado e o erro revisados antes do agente |
| LLM06 Excessive Agency [6] | `excessive-agency`, `approval-forgery`, `tool-shadowing` | Política por agente e servidor; aprovação humana com limites |
| LLM10 Unbounded Consumption [7] | `resource-abuse` | Cota, orçamento de custo e disjuntor |

## 6. Como reproduzir

```sh
git checkout cap22
go test -race -count=1 -v ./attacks/redteam
```

## Referências

1. SCARFONE, K. et al. *Technical Guide to Information Security Testing and Assessment*. NIST SP 800-115, 2008. <https://csrc.nist.gov/pubs/sp/800/115/final>
2. OWASP. *LLM01:2025 Prompt Injection*. <https://genai.owasp.org/llmrisk/llm01-prompt-injection/>
3. OWASP. *LLM02:2025 Sensitive Information Disclosure*. <https://genai.owasp.org/llmrisk/llm022025-sensitive-information-disclosure/>
4. OWASP. *LLM03:2025 Supply Chain*. <https://genai.owasp.org/llmrisk/llm032025-supply-chain/>
5. OWASP. *LLM05:2025 Improper Output Handling*. <https://genai.owasp.org/llmrisk/llm052025-improper-output-handling/>
6. OWASP. *LLM06:2025 Excessive Agency*. <https://genai.owasp.org/llmrisk/llm062025-excessive-agency/>
7. OWASP. *LLM10:2025 Unbounded Consumption*. <https://genai.owasp.org/llmrisk/llm102025-unbounded-consumption/>
