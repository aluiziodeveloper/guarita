// Package sandbox reproduz a fuga do Capítulo 17: um servidor de
// arquivos que segue um symlink para fora do diretório permitido, e
// uma sonda que tenta, de dentro de onde roda, escrever no sistema,
// conectar ao host, ler um arquivo do host, usar o chaveiro do kernel
// e virar root. Os testes exigem um Docker local e são pulados sem
// ele; na CI, o Docker do runner os roda.
//
// O segredo é um arquivo temporário com um marcador [LAB], e a conexão
// é com um ouvinte do próprio teste.
package sandbox
