# ADR 0020: Padrões seguros, escuta de operação e implantação sem privilégios

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 21; a Guarita sai do laboratório e vai para produção

## Contexto

Até o Capítulo 20, os padrões eram os de um laboratório. No modo HTTP, sem `-agent-key`, a Guarita aceitava o agente sem token no loopback, com um aviso no registro; fora do loopback, exigia a identidade, mas servia em claro, e o token atravessava a rede legível. Não havia medida de latência, nem saúde, nem perfil, e o exemplo ingênuo do capítulo mostra o atalho comum: importar `net/http/pprof` e servir o `DefaultServeMux`, o que põe os perfis e a linha de comando do processo na porta do serviço. A soma das defesas dos Capítulos 12 a 19 nunca tinha sido medida contra um orçamento.

## Decisão

1. **Identidade e TLS por padrão.** Sem `-agent-key`, a partida falha, a menos que `-insecure-no-auth` esteja escrito, e ele só vale no loopback. Com `-host`, a Guarita exige `-agent-key`, `-tls-cert` e `-tls-key`, e serve TLS 1.3 ou mais novo. O recurso protegido usa o nome público de `-host`.
2. **Escuta de operação separada.** `-ops-listen` serve `/metrics`, `/healthz` e `/readyz` num multiplexador próprio, e é vazia por padrão. No loopback, a guarda de Host vale também ali. `-pprof` registra os perfis um a um, e só é aceito no loopback. `/readyz` vira 503 no encerramento.
3. **Métricas sem dependência nova.** Histogramas atômicos no formato de texto do Prometheus: o pedido, o servidor e a diferença, que é a latência acrescentada pela Guarita. As medidas não levam argumentos, nomes de ferramentas nem identidades.
4. **SLO de 5 ms no p99**, com a política, os inspetores determinísticos e as métricas, conferido por `TestSLO` na CI. A trilha e o classificador ficam fora dele.
5. **Imagem distroless** e manifestos de Compose e Kubernetes sem privilégios: só leitura, como 65532, sem capabilities nem escalada, com limites e probes; no Kubernetes, NetworkPolicy.
6. **Segredo do orquestrador.** O grupo pode ler um segredo só se o arquivo for do root e o grupo for o do processo, a forma com que o kubelet entrega um Secret com `fsGroup`. No Compose sem Swarm, que ignora `uid`, `gid` e `mode`, um serviço de inicialização entrega os segredos assim.

## Alternativas consideradas

1. **`prometheus/client_golang` para as métricas.** Rejeitada por ora: é a escolha comum, mas acrescentaria uma árvore de dependências ao binário que o Capítulo 20 passou a inventariar, para cinco medidas. A saída já segue o formato dele, e a troca não muda quem coleta.
2. **Métricas do OpenTelemetry**, já usado nos traces. Adiada: o exportador para Prometheus é outra dependência, e as medidas do capítulo são poucas.
3. **Manter o agente sem token como padrão no loopback.** Rejeitada: um processo qualquer da máquina vira agente, e o padrão de um laboratório acaba em produção.
4. **Afrouxar a regra do segredo para todo arquivo legível pelo grupo do processo.** Rejeitada: num host, o grupo de um usuário pode ser compartilhado; a forma do orquestrador, com o arquivo do root, é estreita o bastante.
5. **`pprof` com autenticação fora do loopback.** Fora do escopo: o perfil se pede por túnel ou `kubectl port-forward`.

## Consequências

- O perfil de CPU sob carga mostrou as expressões regulares do inspetor de injeção em 13,8% do tempo; o descarte por `strings.Contains` levou a etapa de 68,3 µs para 12,7 µs e a vazão do processo de 6549 para 7201 chamadas por segundo, nas medianas de três execuções.
- No laboratório, a Guarita acrescentou no p99 0,43 ms com a política, 0,62 ms com os inspetores e 1,37 ms com a trilha.
- A imagem tem 35,7 MB, contra 885,8 MB da imagem do Go.
- A implantação achou dois defeitos, corrigidos: o recurso de uma rota nomeada usava o endereço da escuta, e a regra do segredo recusava o Secret do Kubernetes.
- Todo laboratório e todo teste do modo HTTP sem token passam a escrever `-insecure-no-auth`.
- A escuta de operação fora do loopback não tem autenticação: quem a alcança lê as medidas. A NetworkPolicy, ou a rede do host, precisa restringi-la.
