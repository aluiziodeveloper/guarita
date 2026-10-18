# Modelo de ameaças da Guarita — v1

- **Versão:** v1, Capítulo 3 do livro (*tag* `cap03`); ameaça `dns-rebinding` acrescentada no Capítulo 7 (*tag* `cap07`), quando a Guarita passou a escutar na rede; `credential-theft` parcialmente mitigada no Capítulo 8 (*tag* `cap08`); `rug-pull` parcialmente mitigada no Capítulo 9 (*tag* `cap09`); `tool-poisoning` e `tool-shadowing` parcialmente mitigadas no Capítulo 10 (*tag* `cap10`); `excessive-agency` parcialmente mitigada no Capítulo 11 (*tag* `cap11`); `approval-forgery` parcialmente mitigada no Capítulo 12 (*tag* `cap12`); `data-exfiltration` parcialmente mitigada no Capítulo 13 (*tag* `cap13`); `indirect-prompt-injection` parcialmente mitigada no Capítulo 14 (*tag* `cap14`), com o classificador local acrescentado no Capítulo 15 (*tag* `cap15`).
- **Método:** zonas e fronteiras de confiança, STRIDE por ameaça e árvores de ataque em [`trees.md`](trees.md).
- **Fonte de verdade:** a tabela de ameaças deste arquivo e a função `threatmodel.Model()` precisam coincidir; o teste `TestModelo_coincideComODocumento` quebra a CI quando divergem.

## Escopo

A Guarita é um proxy MCP entre um agente e os servidores MCP que ele usa. Na v1, ela ainda não intercepta nada: o modelo descreve o sistema que o livro vai construir, para que cada defesa tenha uma ameaça e um capítulo antes de existir código de transporte.

Fora do escopo: a segurança do modelo de linguagem em si, do sistema operacional do host e de serviços de terceiros alcançados pelos servidores MCP.

## Ativos

| Ativo | Por que importa |
|---|---|
| Credenciais da Guarita para os servidores | Quem as controla age em nome da Guarita |
| *Tokens* do usuário | Não podem sair da fronteira em que foram emitidos |
| Catálogo aprovado de ferramentas | Base das decisões de política |
| Argumentos e respostas das chamadas | Podem conter dados pessoais e segredos |
| Políticas | Decidem o que o agente pode fazer |
| Trilha de auditoria | Prova do que aconteceu |
| Disponibilidade e cota | Laços de chamadas consomem recursos e dinheiro |

## Zonas de confiança

O identificador de cada zona é o valor de `TrustZone.String()`.

| Zona | Identificador | Confiança |
|---|---|---|
| Agente | `agent` | Não confiável: segue o que o modelo propõe |
| Guarita | `guarita` | Confiável: é onde as decisões são aplicadas |
| Servidor MCP | `mcp-server` | Não confiável: escrito e operado por terceiros |
| Operador humano | `operator` | Confiável para aprovar, sujeito a fadiga e engano |
| Modelo local | `local-model` | Não confiável: usado como classificador a partir do Capítulo 15 |
| Armazenamento | `storage` | Confiável, mas precisa resistir a adulteração |

## Fronteiras de confiança

Uma fronteira é escrita `origem->destino`, como `TrustBoundary.String()`.

| Fronteira | O que atravessa |
|---|---|
| `agent->guarita` | Pedidos de chamada, argumentos gerados pelo modelo, mensagens do protocolo |
| `guarita->mcp-server` | Chamadas repassadas, credenciais e o processo isolado do servidor |
| `mcp-server->guarita` | Definições de ferramentas e respostas das chamadas |
| `operator->guarita` | Aprovações de chamadas pausadas |
| `operator->mcp-server` | Instalação e atualização de servidores |

## Ameaças

Estado `open` significa que a Guarita ainda não tem a defesa. A conferência opt-in de descrições do agente de laboratório (Capítulo 1) fica fora da Guarita e não muda nenhum estado.

| ID | Ameaça | STRIDE | Fronteira | Capítulo | Estado |
|---|---|---|---|---|---|
| `tool-poisoning` | Envenenamento de ferramenta | tampering, elevation-of-privilege | mcp-server->guarita | 10 | partially-mitigated |
| `rug-pull` | Troca silenciosa da definição aprovada | tampering | mcp-server->guarita | 9 | partially-mitigated |
| `tool-shadowing` | Sombreamento de ferramenta por nome repetido | spoofing | mcp-server->guarita | 10 | partially-mitigated |
| `indirect-prompt-injection` | Injeção de *prompt* indireta na resposta | tampering, elevation-of-privilege | mcp-server->guarita | 14 | partially-mitigated |
| `excessive-agency` | Agência excessiva | elevation-of-privilege | agent->guarita | 11 | partially-mitigated |
| `data-exfiltration` | Exfiltração de dados nos argumentos | information-disclosure | guarita->mcp-server | 13 | partially-mitigated |
| `sandbox-escape` | Fuga da sandbox do servidor | elevation-of-privilege | guarita->mcp-server | 17 | open |
| `credential-theft` | Repasse ou roubo de credenciais (*confused deputy*) | spoofing, information-disclosure | guarita->mcp-server | 8 | partially-mitigated |
| `supply-chain` | Pacote de servidor comprometido | tampering | operator->mcp-server | 20 | open |
| `resource-abuse` | Laços de chamadas que esgotam cota ou custo | denial-of-service | agent->guarita | 18 | open |
| `insecure-parsing` | Mensagem malformada ou ambígua no protocolo | tampering, denial-of-service | agent->guarita | 6 | partially-mitigated |
| `approval-forgery` | Aprovação forjada ou obtida por fadiga | spoofing, repudiation | operator->guarita | 12 | partially-mitigated |
| `dns-rebinding` | Página da web alcança a Guarita local por DNS rebinding | spoofing, elevation-of-privilege | agent->guarita | 7 | partially-mitigated |

## Defesas em vigor

| ID | Desde | O que cobre | Testes de ataque | O que falta |
|---|---|---|---|---|
| `insecure-parsing` | Capítulo 6 | Parser estrito das mensagens do agente: um valor por mensagem, sem chave repetida, sem lote, UTF-8, aninhamento e tamanho limitados. No stdio desde o Capítulo 6; nos POST do transporte HTTP desde o Capítulo 7, com corpo limitado a 4 MiB e prazo de leitura | `internal/adapters/mcpproxy`: contrabando, chave duplicada, mensagem grande, golden e fuzzing; `internal/adapters/mcphttp`: contrato com contrabando, corpo grande e corpo lento | Respostas do servidor (desafio do Capítulo 6) |
| `dns-rebinding` | Capítulo 7 | `Host` de todo pedido contra a lista derivada do endereço de escuta; `Origin`, quando vem, contra uma lista exata; escuta em loopback por padrão; 403 com erro JSON-RPC | `internal/adapters/mcphttp`: guarda, contrato e integração; `deploy/rebinding-lab.sh` com `curl --resolve` | Fora do loopback, a proteção depende da lista em `-host`; desde o Capítulo 8, a Guarita também exige o *token* do agente nesse caso |
| `credential-theft` | Capítulo 8 | No transporte HTTP: *token* do agente verificado (Ed25519, emissor, audiência igual ao recurso da Guarita, validade); repasse só dos cabeçalhos permitidos; credencial própria da Guarita para o servidor; segredos com tipo que se redige nos registros | `internal/adapters/identity` (audiência, algoritmo, `alg none`); `internal/adapters/mcphttp`: cabeçalhos permitidos, credencial, *confused deputy*, registro sem segredos | Desde o Capítulo 10, cada servidor tem a sua credencial e a sua audiência; escopos por ferramenta e agente na política (Capítulo 11); o transporte stdio não tem identidade de agente |
| `rug-pull` | Capítulo 9 | No transporte HTTP: impressão digital SHA-256 da definição canônica inteira; confiança no primeiro uso; a definição divergente sai de `tools/list`, fica registrada no catálogo em SQLite e a ferramenta é negada em `tools/call`, inclusive se o servidor voltar ao original | `internal/domain` (forma canônica); `internal/app` (catálogo antes da política); `internal/adapters/mcphttp` (JSON e SSE); `attacks/rugpull` | A primeira definição é aceita sem revisão humana; a troca da implementação sem mudar a definição só a *sandbox* contém (Capítulo 17); o transporte stdio ainda não lê `tools/list`; aprovar uma atualização é o desafio do Capítulo 9 |
| `tool-poisoning` | Capítulo 10 | No transporte HTTP, antes do *pin*: a definição inteira (chaves e valores, em qualquer profundidade) é recusada, e não reescrita, se tiver caractere de formatação (Cf: largura zero, controles de direção, *tags* Unicode), de controle fora de quebra de linha e tabulação, texto que muda sob NFKC ou nome fora de `[A-Za-z0-9_.-]{1,128}`; cada recusa vai ao registro com servidor, ferramenta, motivo e sinais | `internal/adapters/scanner`; `internal/app` (varredura antes do *pin*); `attacks/poisoning` (corpus de 16 definições, 4 legítimas) | O varredor é estrutural: texto hostil visível, em letras comuns, passa (classificadores nos Capítulos 14 e 15); marcas combinantes (Mn), como seletores de variação, passam (desafio do Capítulo 10); homóglifos de outros alfabetos fora do nome passam; o transporte stdio ainda não lê `tools/list` |
| `tool-shadowing` | Capítulo 10 | Cada servidor num caminho próprio (`/mcp/{namespace}`), com namespace dado pelo operador; um catálogo só para todos os servidores; ferramenta cuja silhueta (minúsculas ASCII, `-` e `.` como `_`) já foi fixada por outro servidor é recusada: o primeiro fica com o nome | `internal/domain` (silhueta); `internal/adapters/sqlitestore` (migração 0003 e `Owner`); `cmd/guarita` (namespaces); `attacks/shadowing` | Silhuetas por homóglifo não se aplicam, porque o nome já é só ASCII; nomes diferentes com descrições que disputam a mesma tarefa (`read_file` contra `open_document`) passam; o primeiro servidor a anunciar vence, mesmo que seja o hostil |
| `excessive-agency` | Capítulo 11 | Política em Rego, avaliada pelo OPA embutido, com negação por padrão e concessões por agente (o sujeito do *token*) e por servidor (o namespace); regras sobre argumentos que negam o concedido; decisão indefinida, conflito ou política que não compila negam ou impedem a partida; regras testadas com `opa test` na CI | `internal/adapters/policyopa` (adaptador e política versionada); `policies/guarita_test.rego`; `cmd/guarita` (agente e servidor na entrada); `attacks/excessiveagency` | O agente ainda vê em `tools/list` as ferramentas que não pode chamar; `require_approval` só nega até a aprovação humana do Capítulo 12; no stdio não há agente, e só as concessões sem agente valem; as concessões são tão boas quanto quem as escreve, e nada mede se ainda são necessárias |
| `approval-forgery` | Capítulo 12 | Estados da aprovação numa tabela do domínio, com estados terminais (decisão não se repete); identificadores de `rand.Text`; interface do operador noutra escuta, só em loopback, com guarda de Host, `http.CrossOriginProtection` e credencial própria (Bearer ou Basic, comparada em tempo constante); o agente não decide o próprio pedido; prazo com expiração; no máximo três pendentes por agente e a mesma chamada negada recusada na hora por dez minutos | `internal/domain` (transições); `internal/adapters/approval` (prazo, limites, com `synctest`); `internal/adapters/admin` (fluxo, autenticação, escape); `cmd/guarita` (CLI de ponta a ponta); `attacks/approvalfatigue`; `attacks/approvalforgery` | A fila vive em memória e some ao reiniciar; o operador decide sem contexto além da chamada (sem histórico do agente, sem motivo); os segredos dos operadores são estáticos, sem expiração nem MFA; não há registro durável de quem aprovou o quê (auditoria, Capítulo 19); uma aprovação vale para a chamada, e não para um padrão; o agente pode variar os argumentos para escapar da negação repetida |
| `data-exfiltration` | Capítulo 13 | No transporte HTTP, com `-dlp`: inspetor depois da política, com a decisão mais severa vencendo; CPF com dígitos verificadores (formatado ou não), formatos públicos de chave (AWS, GitHub, Slack, chave privada), o mesmo dentro de base64 e tokens de entropia alta; com `-allow-hosts`, URLs só para hosts da lista, pelo `url.Parse`; com `-validate-arguments`, argumentos contra o `inputSchema` fixado, sem `$ref` remoto; achados sem os valores | `internal/adapters/dlp` (detectores, destinos, propriedades com semente fixa, falsos positivos medidos); `internal/adapters/schema`; `internal/app` (inspetores depois da política); `cmd/guarita`; `attacks/exfiltration` | Opt-in: sem `-dlp`, nada é inspecionado; o transporte stdio não tem inspetores; segredos em hexadecimal ficam abaixo do limiar de entropia; dado cifrado, comprimido ou partido em várias chamadas passa; nomes e endereços em texto livre não são detectados; 6 em 1000 números de 11 dígitos passam como CPF; blobs em base64 legítimos pedem aprovação; as respostas do servidor não são inspecionadas (Capítulo 14) |
| `indirect-prompt-injection` | Capítulo 14 | No transporte HTTP, com `-inspect-results`: o resultado de cada `tools/call`, em JSON e em SSE, passa pelo inspetor antes do agente; sinais fortes barram o resultado inteiro, que chega como erro de ferramenta (`isError`): caracteres invisíveis e de controle (menos o U+200D entre emojis), delimitadores de conversa (`[INST]`, `<<SYS>>`, `<\|im_start\|>`) e estilos que escondem texto; sinais fracos (rótulos de papel, comentários HTML) vão ao registro. Com `-mark-untrusted`, o texto de cada resultado vai entre delimitadores com servidor, ferramenta e identificador sorteado por resposta. Desde o Capítulo 15, com `-classifier`, depois das regras, um modelo local no Ollama (padrão `qwen2.5:7b`) dá a probabilidade de cada texto do resultado, em pedaços de 2000 runas, ser instrução para o assistente; a partir de 0,8, o resultado é barrado. O limiar saiu do conjunto de desenvolvimento, com no máximo 5% de falsos positivos; o prazo é de 5 segundos por resultado; o modelo é conferido e aquecido na partida; modelo fora do ar ou lento barra o resultado (`deny`, o padrão) ou devolve a decisão às regras (`rules`), com o achado `classifier-unavailable` | `internal/adapters/injection`; `internal/adapters/untrusted`; `internal/adapters/mcphttp` (JSON e SSE); `internal/app` (`ReviewResult`); `internal/adapters/classifier` (prazo, pedaços, plano B); `internal/adapters/ollama`; `internal/evaluation`; `attacks/injection` (11 *fixtures*, 12 resultados legítimos); `attacks/classifier` (modelo fora do ar, na CI; conjunto de referência com 60 exemplos de desenvolvimento, 60 de teste e 12 ofuscados, só com um Ollama local) | Sem `-classifier`, texto hostil visível, em prosa comum, passa por todos os detectores; com ele, o texto codificado (base64, invertido, ROT13) passa pelo classificador, e um ataque de teste ficou abaixo do limiar; as medidas valem para o modelo, o *prompt* e o conjunto de referência do laboratório, escrito para o livro, e não para o tráfego real; o classificador também lê o texto hostil e pode ser instruído por ele; cada pergunta custa perto de um segundo em CPU; com `rules`, a prosa hostil passa enquanto o modelo estiver fora; a medida do conjunto de referência não roda na CI; os sinais fracos passam marcados, e a marcação só funciona se o modelo for instruído a respeitá-la, o que a Guarita não controla nem mede; blocos que não são texto, `structuredContent` e recursos embutidos não são marcados; o transporte stdio não revisa respostas; as notificações e os erros JSON-RPC passam sem revisão |

## A cena do Capítulo 3: o *confused deputy*

A Guarita guarda credenciais próprias para falar com os servidores (fronteira `guarita->mcp-server`). O agente, que não é confiável, escolhe servidor, ferramenta e argumentos (fronteira `agent->guarita`). Sem uma decisão explícita no meio, o agente consegue fazer a Guarita usar o privilégio dela num pedido que a Guarita jamais faria por conta própria. A ameaça é `credential-theft`, e o desenho já mostra a defesa: credencial por servidor, nunca o *token* do usuário, e política entre a escolha do agente e o uso da credencial (Capítulos 8 e 11).

## Como atualizar

1. Toda ameaça nova entra na tabela e em `threatmodel.Model()` no mesmo commit.
2. O estado só muda para `partially-mitigated` ou `mitigated` com teste de ataque na suíte do capítulo que trouxe a defesa.
3. Mudanças de zona ou de fronteira exigem revisão das árvores em `trees.md`.
