# Roadmap aberto da Guarita

O que a Guarita ainda não faz, na ordem em que fecharia mais risco residual do [relatório de segurança](security-report.md). Nada aqui está prometido para uma data; cada item entra com um teste de ataque em `attacks/`, como os capítulos do livro.

1. **Revisão das respostas no stdio.** Abrir as mensagens do servidor no `mcpproxy.Relay`, como o `mcphttp` já faz, para que um servidor local na sandbox também tenha a volta revisada e marcada.
2. **Sandbox para servidores locais das rotas HTTP.** Uma rota cujo servidor é um processo local, iniciado pela Guarita no perfil do Capítulo 17, para que isolamento e revisão convivam na mesma implantação.
3. **Política para o que não é `tools/call`.** `resources/read` e `prompts/get` passam pela porta sem decisão; a política em Rego ganharia o método, e a revisão, as respostas deles.
4. **Notificações no fluxo da resposta.** Revisar as notificações que o servidor manda no meio da resposta de `tools/call` (o desafio do Capítulo 22 faz o primeiro passo).
5. **Marcação além do texto.** Delimitar ou barrar sinais fracos em `structuredContent`, recursos embutidos e no erro JSON-RPC.
6. **Inspeção com estado.** DLP que junta os argumentos de várias chamadas do mesmo agente; classificador avaliado no tráfego real, com ofuscação.
7. **Cotas compartilhadas.** Baldes num armazenamento comum, para várias Guaritas e reinícios; limite de chamadas simultâneas por agente.
8. **Aprovação do catálogo e da operação.** Aprovar uma definição antes do primeiro uso, no lugar da confiança no primeiro uso; *webhooks* para a fila de aprovação.
9. **Endurecimento do contêiner.** Perfil seccomp próprio; lista de destinos de saída para servidores que precisam de rede.
10. **A versão 1.0.0.** *Release* assinada com registro público de transparência, publicada no dia da edição 1.0 do livro, depois da revisão técnica externa e da reprodução em máquina distinta.
