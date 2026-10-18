# Ataques de laboratório

Cada pacote deste diretório reproduz um ataque contra a Guarita, com um servidor MCP de laboratório e um teste que roda o ataque duas vezes: sem a defesa, ele precisa funcionar; com a defesa, precisa falhar. Um ataque que volte a funcionar quebra o *build*.

Os servidores daqui existem só para o laboratório do livro. Eles rodam dentro dos testes, em `127.0.0.1`, usam marcadores `[LAB]` no lugar de conteúdo nocivo e não devem ser usados fora deste repositório.

| Pacote | Ataque | Defesa | Capítulo |
|---|---|---|---|
| `rugpull` | O servidor troca o esquema de uma ferramenta depois da primeira chamada | Catálogo com *pin* pela impressão digital | 9 |
| `poisoning` | Definições de ferramenta com instrução disfarçada | Varredor estrutural das definições | 10 |
| `shadowing` | Um segundo servidor anuncia uma ferramenta com a silhueta do nome de outra | *Namespaces* por servidor e silhueta do nome | 10 |
| `excessiveagency` | A lista global de permitidas entrega uma ferramenta destrutiva a um agente que só precisava ler | Política em Rego por agente e servidor | 11 |
| `approvalfatigue` | O agente repete o pedido negado até alguém ceder | Limite de pendentes e recusa da mesma chamada negada | 12 |
| `approvalforgery` | O agente, ou uma página no navegador do operador, tenta aprovar o próprio pedido | Interface do operador separada, com credencial e proteção de origem | 12 |
| `exfiltration` | CPF, chave ou destino de fora nos argumentos de uma ferramenta permitida | Inspetores de DLP e de destinos | 13 |
| `injection` | O resultado de uma busca traz texto escrito para o modelo | Revisão do resultado: sinais fortes barram, o resto chega marcado | 14 |
| `classifier` | Injeção em prosa comum e ofuscada; o modelo local fora do ar | Classificador local medido no conjunto de referência, com prazo e plano B | 15 |
| `pipeline` | Uma página em base64 passa pela volta, e a rota de envio, sem linha, deixa o CPF sair | Linha de inspetores por rota, com teste de camada ausente | 16 |
| `sandbox` | Um servidor de arquivos segue um symlink para fora da raiz; uma sonda tenta escrever, conectar, ler o host, usar o chaveiro do kernel e virar root | `os.Root` no servidor e o contêiner da Guarita, sem rede, só leitura e sem privilégios | 17 |
| `abuse` | Um agente em laço esgota a fatura de um modelo pago; um servidor que falha devagar segura cada chamada | Cota e orçamento de custo por agente; disjuntor no repasse | 18 |

A medida do classificador (`attacks/classifier`, `TestClassificador`) exige um Ollama local e é pulada sem ele; na CI, só o teste do modelo fora do ar roda. Os testes de `attacks/sandbox` exigem um Docker local; na CI, o Docker do runner os roda. Para medir:

```sh
docker run -d --name guarita-ollama -p 127.0.0.1:11434:11434 \
  -v guarita-ollama:/root/.ollama ollama/ollama:0.35.1
docker exec guarita-ollama ollama pull qwen2.5:7b
GUARITA_OLLAMA=http://127.0.0.1:11434 \
  go test -count=1 -v -run TestClassificador ./attacks/classifier
```
