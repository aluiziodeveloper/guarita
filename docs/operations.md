# Guia de operação da Guarita

Este guia é o do Capítulo 21: como subir a Guarita em produção, o que observar e o que fazer quando algo sai do esperado. Os números são os do laboratório do livro, num Intel Core i7-13700H; meça os seus.

## Padrões

| Situação | Padrão | Para mudar |
|---|---|---|
| Agente sem token | Recusado na partida | `-insecure-no-auth`, só em loopback, para laboratório |
| Escuta fora do loopback (`-host`) | Exige `-agent-key` e TLS | `-tls-cert` e `-tls-key`; TLS 1.3 ou mais novo |
| Escuta de operação | Nenhuma | `-ops-listen endereço` |
| Perfis do `pprof` | Desligados | `-pprof`, só com `-ops-listen` no loopback |
| Trilha de auditoria | Desligada | `-audit` e `-audit-key` |
| Segredo legível pelo grupo | Recusado | Aceito só do root para o grupo do processo, como entrega um orquestrador |

## Implantação

- **Imagem:** `deploy/guarita.Dockerfile`, distroless, estática, sem shell, como 65532, com as políticas em `/policies`.
- **Docker Compose:** `deploy/prod/compose.yaml`; `deploy/prod/lab.sh` gera os segredos de laboratório, sobe, chama com e sem token, lê as métricas e verifica a trilha.
- **Kubernetes:** `deploy/k8s/guarita.yaml`; `deploy/k8s/kind-lab.sh` faz o mesmo num cluster do kind. Troque o `emptyDir` da trilha por um volume persistente, e o certificado de laboratório pelo do seu emissor.

Em todos, a Guarita roda só leitura, sem capabilities nem escalada de privilégios, com limites de CPU e memória, e o servidor MCP fica no loopback dela, sem porta própria.

## O que observar

`-ops-listen` expõe, no formato de texto do Prometheus:

| Medida | O que diz |
|---|---|
| `guarita_overhead_seconds` | O tempo de cada pedido que não foi do servidor: a latência que a Guarita acrescenta |
| `guarita_request_seconds` | O pedido inteiro |
| `guarita_upstream_seconds` | A ida e volta ao servidor, até os cabeçalhos |
| `guarita_messages_total{outcome}` | Mensagens repassadas e negadas |
| `guarita_in_flight` | Pedidos em curso |

`/healthz` responde enquanto o processo atende; `/readyz`, só enquanto ele aceita pedidos, e vira 503 no encerramento. `guarita health` é o probe para a imagem sem shell.

## SLO

O objetivo é que a Guarita acrescente no máximo 5 ms no p99, com a política, os inspetores determinísticos e as métricas. `TestSLO` (`GUARITA_SLO=1 go test -run TestSLO ./cmd/guarita`) mede isso a cada push na CI. Na execução gravada no laboratório, o acréscimo no p99 foi de 0,41 a 0,69 ms nessas configurações; com a trilha, que espera o `fsync`, de 2,62 ms. Os números variam de uma execução para outra, e o classificador local do Capítulo 15 fica fora do orçamento, porque leva segundos.

Em produção, acompanhe a fração dos pedidos com `guarita_overhead_seconds` acima de 5 ms: com o histograma, é `1 - bucket{le="0.005"} / count`, e o orçamento de erro de um SLO de 99% é 1% dos pedidos.

## Perfil sob carga

```sh
guarita … -ops-listen 127.0.0.1:9090 -pprof
go tool pprof -top http://127.0.0.1:9090/debug/pprof/profile?seconds=10
```

O `lab-load` (`cmd/lab-load`) põe carga no laboratório: `-serve` é um servidor de mentira, e `-target` manda chamadas em paralelo.

## Diagnóstico rápido

| Sintoma | Causa provável |
|---|---|
| `-agent-key is required; -insecure-no-auth …` | Modo HTTP sem identidade: dê a chave do emissor ou, em laboratório, a opção |
| `-tls-cert and -tls-key are required when -host is set` | Escuta pública sem TLS |
| `-pprof requires a loopback -ops-listen` | Perfis pedidos numa escuta pública |
| `secret file is readable by others` | Segredo legível pelos demais, ou pelo grupo sem ser do root |
| Token recusado atrás de um nome público | O `aud` do token precisa ser o recurso de `-host`, com o esquema |
| Pod em `CrashLoopBackOff` | `kubectl logs`: a Guarita diz na partida o que falta |
