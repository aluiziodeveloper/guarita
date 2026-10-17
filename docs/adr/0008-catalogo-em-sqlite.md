# ADR 0008: Catálogo de ferramentas com pin por impressão digital, em SQLite

- **Estado:** aceita
- **Data:** 2026-10-17
- **Contexto:** Capítulo 9; primeira persistência da Guarita

## Contexto

A definição de uma ferramenta (nome, descrição, esquemas, anotações) chega ao modelo como instrução. Um servidor pode trocá-la depois de aprovado, sem mudar o nome: é o *rug pull*. Até o Capítulo 8, a Guarita repassava `tools/list` sem ler.

Duas fixações ingênuas falham (`examples/naivepin`): fixar só a descrição deixa passar a instrução escondida na descrição de um parâmetro; fixar os bytes como chegaram acusa mudança quando o servidor só reordena as chaves.

## Decisão

1. A impressão digital é o SHA-256 da forma canônica do objeto JSON inteiro da ferramenta: chaves em ordem, sem espaços, números como vieram. Qualquer campo, inclusive os que a especificação acrescentar, entra no resumo.
2. A forma canônica é a de `json.Marshal` sobre o valor decodificado com `UseNumber`. Ela não é a JCS (RFC 8785): é determinística para a Guarita, que só compara os próprios resumos.
3. Confiança no primeiro uso: a primeira definição vista de cada ferramenta, por servidor, fica fixada.
4. A definição divergente sai de `tools/list`, fica registrada ao lado da fixada, e a ferramenta é negada em `tools/call`, antes da política, até o operador decidir. Voltar ao original não a libera.
5. O catálogo fica em SQLite, com `modernc.org/sqlite` (Go puro, sem cgo, compatível com a imagem distroless). As migrações são arquivos `.sql` embutidos com `embed`, aplicados em ordem ao abrir, cada um numa transação com `PRAGMA user_version`.
6. Consultas sempre com parâmetros; nunca com valores concatenados.
7. Falha fechada: resposta de `tools/list` ilegível ou catálogo indisponível viram 502, e nada chega ao agente.
8. O caso de uso `PinCatalog` depende da porta `CatalogStore`; o domínio só calcula a impressão digital, sem E/S.

## Alternativas consideradas

1. **Exigir aprovação humana antes do primeiro uso.** Adiada: exige o fluxo de aprovação do Capítulo 12. A confiança no primeiro uso fecha a troca posterior, que é o *rug pull*.
2. **Banco de dados em servidor (PostgreSQL).** Rejeitada para o livro: o catálogo é pequeno e local, e um arquivo único não exige operação.
3. **`mattn/go-sqlite3`.** Rejeitada: exige cgo, que a imagem do laboratório não tem.
4. **Tabela `schema_migrations` no lugar de `user_version`.** Rejeitada: o SQLite já guarda um inteiro de versão no cabeçalho do arquivo.
5. **JCS completa.** Adiada: só seria necessária se outro sistema precisasse reproduzir o resumo.

## Consequências

- Uma atualização legítima do servidor barra a ferramenta até o operador aprovar; o desafio do Capítulo 9 acrescenta `ApproveUpdate`.
- A primeira definição é aceita sem revisão: um servidor que já chega envenenado passa pelo catálogo. A varredura das descrições é o Capítulo 10.
- Trocar a implementação sem trocar a definição não é visível aqui; a *sandbox* do Capítulo 17 contém esse caso.
- O catálogo existe no transporte HTTP; no stdio, a leitura de `tools/list` fica para depois.
- O mesmo banco vai receber a auditoria no Capítulo 19, com migrações novas.
