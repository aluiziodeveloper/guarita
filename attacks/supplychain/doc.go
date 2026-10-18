// Package supplychain reproduz, no laboratório do Capítulo 20, a
// atualização de um servidor MCP que traz código fora da versão
// aprovada: o servidor de arquivos compilado com a etiqueta labupdate
// anuncia o mesmo nome, a mesma versão e as mesmas ferramentas, e roda
// um trecho a mais que só escreve um marcador neutro. Nada sai da
// máquina, e os binários são compilados pelo próprio teste.
package supplychain
