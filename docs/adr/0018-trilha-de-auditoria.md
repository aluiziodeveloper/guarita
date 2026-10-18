# ADR 0018: Trilha de auditoria encadeada por hashes, com HMAC dos argumentos e trace W3C

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 19; adulteração do registro e investigação depois de um incidente

## Contexto

O registro da Guarita é feito com `log/slog` em JSON, e o registro ingênuo de chamadas (`examples/naiveaudit`) põe uma linha por chamada num arquivo, com os argumentos. Depois de um incidente, ninguém consegue provar o que foi decidido: uma negação trocada por permissão com um editor de texto continua um JSON válido, e o arquivo guarda o CPF de exemplo que o agente mandou. Os capítulos anteriores também deixaram lacunas que dependem de um registro: quem aprovou, qual chamada foi barrada por cota, qual servidor recebeu o quê.

## Decisão

1. O domínio define `AuditEntry` e o encadeamento: cada entrada guarda o número, o SHA-256 da anterior (a primeira, um valor fixo) e o próprio SHA-256, sobre o JSON da entrada com o campo do hash vazio. O JSON de uma estrutura sai com os campos na ordem da declaração, e o mesmo conteúdo dá o mesmo hash.
2. A trilha (`internal/adapters/audit`) é um arquivo JSON Lines aberto só para acréscimo, com `fsync` a cada entrada. Na abertura, a trilha existente é verificada, e uma trilha quebrada não é estendida. O verificador exige que cada linha seja, byte a byte, a forma canônica da entrada.
3. O caso de uso registra cada decisão de `Authorize`, permitida ou negada, antes de a chamada seguir, com o motivo da negação ou a indicação de aprovação. Se o registro falhar, a chamada é negada.
4. Os argumentos entram só como HMAC-SHA256, com uma chave do operador. Um hash sem chave revelaria os valores de poucas possibilidades, como um CPF, a quem tivesse a trilha.
5. O transporte HTTP extrai o `traceparent` do W3C Trace Context, abre um span da Guarita pelo OpenTelemetry (v1.47.0) e o injeta no pedido ao servidor; a trilha grava o `trace_id`, e `-trace-file` exporta os spans em JSON.
6. O registro de operação troca por `[redigido]` os atributos de nome sensível, por um `ReplaceAttr` do `slog`.
7. `guarita audit verify [-head]` diz onde a trilha quebra e, com a cabeça guardada à parte, acusa a cadeia refeita e o fim cortado.

## Alternativas consideradas

1. **Assinar cada entrada.** Adiada para o desafio do capítulo: uma assinatura por entrada custa mais e não resolve o fim cortado; assinar a cabeça, de tempos em tempos, resolve os dois casos que a cadeia sozinha não acusa.
2. **Um banco de dados para a trilha, como o SQLite do catálogo.** Rejeitada: um arquivo só de acréscimo, uma linha por entrada, é o formato que qualquer ferramenta lê e que o verificador confere linha por linha, sem esquema.
3. **Gravar os argumentos cifrados.** Rejeitada: a trilha passaria a guardar o dado sensível, só que cifrado, e a chave viraria a chave de todos os dados; o HMAC permite conferir sem guardar.
4. **Um serviço externo de registro imutável.** Fora do escopo do livro; a cabeça guardada à parte é o primeiro passo dele.

## Consequências

- No laboratório, a cadeia sozinha acusou cinco das sete adulterações (decisão trocada, hash de uma linha refeito, linha apagada, linhas trocadas e campo a mais); a cadeia refeita do meio e o fim cortado, só a cabeça.
- Cada decisão espera o `fsync` do disco, e o tempo dele entra em cada chamada.
- A trilha fica na mesma máquina, com as permissões do usuário da Guarita.
- A trilha diz que a chamada foi aprovada, mas não por qual operador.
- O HMAC depende de o JSON dos argumentos chegar com as chaves na mesma ordem.
