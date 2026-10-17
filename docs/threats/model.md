# Modelo de ameaças da Guarita — v1

- **Versão:** v1, Capítulo 3 do livro (*tag* `cap03`); ameaça `dns-rebinding` acrescentada no Capítulo 7 (*tag* `cap07`), quando a Guarita passou a escutar na rede; `credential-theft` parcialmente mitigada no Capítulo 8 (*tag* `cap08`); `rug-pull` parcialmente mitigada no Capítulo 9 (*tag* `cap09`); `tool-poisoning` e `tool-shadowing` parcialmente mitigadas no Capítulo 10 (*tag* `cap10`); `excessive-agency` parcialmente mitigada no Capítulo 11 (*tag* `cap11`).
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
| `indirect-prompt-injection` | Injeção de *prompt* indireta na resposta | tampering, elevation-of-privilege | mcp-server->guarita | 14 | open |
| `excessive-agency` | Agência excessiva | elevation-of-privilege | agent->guarita | 11 | partially-mitigated |
| `data-exfiltration` | Exfiltração de dados nos argumentos | information-disclosure | guarita->mcp-server | 13 | open |
| `sandbox-escape` | Fuga da sandbox do servidor | elevation-of-privilege | guarita->mcp-server | 17 | open |
| `credential-theft` | Repasse ou roubo de credenciais (*confused deputy*) | spoofing, information-disclosure | guarita->mcp-server | 8 | partially-mitigated |
| `supply-chain` | Pacote de servidor comprometido | tampering | operator->mcp-server | 20 | open |
| `resource-abuse` | Laços de chamadas que esgotam cota ou custo | denial-of-service | agent->guarita | 18 | open |
| `insecure-parsing` | Mensagem malformada ou ambígua no protocolo | tampering, denial-of-service | agent->guarita | 6 | partially-mitigated |
| `approval-forgery` | Aprovação forjada ou obtida por fadiga | spoofing, repudiation | operator->guarita | 12 | open |
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

## A cena do Capítulo 3: o *confused deputy*

A Guarita guarda credenciais próprias para falar com os servidores (fronteira `guarita->mcp-server`). O agente, que não é confiável, escolhe servidor, ferramenta e argumentos (fronteira `agent->guarita`). Sem uma decisão explícita no meio, o agente consegue fazer a Guarita usar o privilégio dela num pedido que a Guarita jamais faria por conta própria. A ameaça é `credential-theft`, e o desenho já mostra a defesa: credencial por servidor, nunca o *token* do usuário, e política entre a escolha do agente e o uso da credencial (Capítulos 8 e 11).

## Como atualizar

1. Toda ameaça nova entra na tabela e em `threatmodel.Model()` no mesmo commit.
2. O estado só muda para `partially-mitigated` ou `mitigated` com teste de ataque na suíte do capítulo que trouxe a defesa.
3. Mudanças de zona ou de fronteira exigem revisão das árvores em `trees.md`.
