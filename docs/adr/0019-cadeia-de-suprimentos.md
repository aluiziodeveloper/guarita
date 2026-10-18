# ADR 0019: Servidor fixado pelo digest e release reproduzível, com SBOM e assinatura

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 20; atualização de servidor com código fora da versão aprovada, e a origem do próprio binário da Guarita

## Contexto

Até o Capítulo 19, a Guarita fixava o que um servidor anuncia: as definições das ferramentas, pelo catálogo do Capítulo 9. O código do servidor ficava de fora. No laboratório, uma atualização do servidor de arquivos, compilada com uma etiqueta de build a mais, anunciou o mesmo nome, a mesma versão e as mesmas definições, com o mesmo digest no catálogo, e rodou código que a versão aprovada não tinha. O nome do comando também era procurado no `PATH`, e um diretório novo antes dele trocava o servidor sem mexer no arquivo aprovado.

Do lado da Guarita, o binário não dizia de onde veio, e a CI não conferia as vulnerabilidades conhecidas das dependências nem da biblioteca padrão. O `govulncheck`, rodado pela primeira vez neste capítulo, apontou 12 vulnerabilidades do go1.27.1 alcançáveis pelo código.

## Decisão

1. O domínio ganha `Digest`, o SHA-256 de um artefato no formato `sha256:…`, calculado por partes.
2. O modo stdio aceita `-server-digest`. No host, o nome é resolvido pelo `PATH` e pelos links até o arquivo real, o arquivo é conferido, e é ele, pelo caminho absoluto, que o sistema executa. Com `-sandbox`, a imagem precisa vir referida pelo digest aprovado, e quem confere os bytes é o Docker. `guarita digest` dá o digest para o operador aprovar.
3. O módulo fixa o toolchain com a diretiva `toolchain`, e a CI roda o `govulncheck`, que falha só com vulnerabilidades alcançáveis.
4. A release é montada pelo GoReleaser com `-trimpath`, `CGO_ENABLED=0` e a data do commit nos binários e nos arquivos dos pacotes. A CI a monta duas vezes, em caminhos diferentes, e compara as somas.
5. Cada pacote ganha uma SBOM SPDX, pelo syft, e `checksums.txt` é assinado pelo cosign. `deploy/release/verify.sh` confere a assinatura e só então as somas.
6. `guarita version` mostra a versão gravada por `-ldflags -X` e a informação de build do binário.
7. As *actions* da CI são fixadas pelo hash do commit e as imagens de base pelo digest, e o Dependabot propõe as atualizações.

## Alternativas consideradas

1. **Confiar na versão que o servidor declara no `initialize`.** Rejeitada: é o que a atualização do laboratório repetiu.
2. **Copiar o executável conferido para um diretório só da Guarita e executar a cópia.** Adiada para o desafio do capítulo: fecha a janela entre conferir e executar, ao custo de uma cópia por partida.
3. **Assinatura sem chave (*keyless*) do Sigstore, com o certificado da identidade da CI e o registro no Rekor.** É o caminho de uma release pública. O laboratório usa uma chave local e uma configuração sem log de transparência, para nada sair da máquina, e a verificação precisa de `--insecure-ignore-tlog`.
4. **Atestação de proveniência SLSA gerada pela CI.** Fora do escopo do livro; a release reproduzível é o primeiro passo dela, porque qualquer um pode recompilar e comparar.

## Consequências

- No laboratório, a atualização com as mesmas definições, o arquivo trocado e o mesmo nome antes no `PATH` foram recusados antes de o servidor rodar.
- O `govulncheck` foi de 12 vulnerabilidades alcançáveis a nenhuma, com o go1.27.2 e o `x/crypto` v0.56.0; continua listada uma do `x/crypto/openpgp`, sem correção, que o código não chama.
- O mesmo commit, em dois caminhos, deu os mesmos quatro binários e os mesmos quatro pacotes; as SBOMs diferem na data de criação e no identificador do documento, e não nos pacotes que listam. Sem `-trimpath`, o `go build` comum deu binários diferentes.
- A primeira versão de `verify.sh` passava uma assinatura inválida, porque o status de um *pipeline* é o do último comando. Corrigida antes do commit, e registrada como saída.
- Toda atualização de um servidor exige um digest novo, aprovado por alguém que examinou a versão.
