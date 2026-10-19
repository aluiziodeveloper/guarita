# Registros de decisão de arquitetura

Cada ADR registra uma decisão, o contexto em que foi tomada e as alternativas descartadas. Uma ADR não se reescreve: quando um capítulo posterior muda o que ela diz, ela ganha uma seção de revisão, e a decisão nova, se houver, ganha uma ADR própria. A revisão do Capítulo 22 releu as vinte primeiras à luz do *red team*.

| ADR | Decisão | Capítulo | Estado | Revisão do Capítulo 22 |
|---|---|---:|---|---|
| [0001](0001-proxy-mcp.md) | Guarita como proxy MCP | 1 | aceita para a implementação didática | Confirmada |
| [0002](0002-modelo-de-ameacas-antes-do-transporte.md) | Modelo de ameaças antes do transporte | 3 | aceita | Confirmada; modelo na v2 |
| [0003](0003-portas-e-adaptadores.md) | Portas no consumidor e regra de dependência verificada | 4 | aceita | Confirmada |
| [0004](0004-dono-de-goroutine.md) | Toda goroutine tem dono e termina com o contexto | 5 | aceita | Confirmada |
| [0005](0005-isolamento-do-protocolo.md) | O protocolo MCP numa camada anticorrupção | 6 | aceita | Confirmada |
| [0006](0006-transporte-http.md) | Transporte HTTP com proxy reverso, guarda de Host e prazos | 7 | aceita | Confirmada |
| [0007](0007-identidade-e-credenciais.md) | Identidade do agente e credencial própria da Guarita | 8 | aceita | Confirmada |
| [0008](0008-catalogo-em-sqlite.md) | Catálogo com *pin* por impressão digital, em SQLite | 9 | aceita | Confirmada |
| [0009](0009-varredor-e-namespaces.md) | Varredor estrutural antes do *pin* e um namespace por servidor | 10 | aceita | Confirmada |
| [0010](0010-opa-embutido.md) | Políticas em Rego, com o OPA embutido | 11 | aceita | Revisada: OPA 1.21.2 |
| [0011](0011-aprovacao-humana.md) | Aprovação humana com fila em memória | 12 | aceita | Confirmada |
| [0012](0012-inspecao-deterministica.md) | Inspeção determinística dos argumentos | 13 | aceita | Revisada: também no stdio |
| [0013](0013-revisao-das-respostas.md) | Revisão do resultado e marcação | 14 | aceita | Revisada: o erro JSON-RPC é revisado |
| [0014](0014-classificador-local.md) | Classificador local de injeção | 15 | aceita | Confirmada |
| [0015](0015-linha-de-inspetores-por-rota.md) | Linha de inspetores por rota | 16 | aceita | Revisada: a linha de ida no stdio |
| [0016](0016-sandbox-do-servidor-stdio.md) | Sandbox do servidor stdio | 17 | aceita | Revisada: a sandbox não convive com a revisão das respostas |
| [0017](0017-cotas-e-disjuntor.md) | Cotas, orçamento de custo e disjuntor | 18 | aceita | Revisada: cotas também no stdio |
| [0018](0018-trilha-de-auditoria.md) | Trilha encadeada por *hashes* | 19 | aceita | Confirmada |
| [0019](0019-cadeia-de-suprimentos.md) | Servidor fixado pelo digest e *release* reproduzível | 20 | aceita | Confirmada; a 1.0.0 fica para a edição 1.0 |
| [0020](0020-padroes-seguros-e-operacao.md) | Padrões seguros e operação | 21 | aceita | Confirmada; estendida pela 0021 |
| [0021](0021-red-team-e-configuracao-fechada.md) | *Red team* como regressão e configuração em falha fechada | 22 | aceita | — |
