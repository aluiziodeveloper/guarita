// Package pipeline reproduz a lacuna de cobertura do Capítulo 16: duas
// rotas com linhas de inspetores diferentes, uma busca que traz texto
// para o modelo e um envio que leva dados para fora. Cada ataque tem
// uma camada que o para, e o teste de camada ausente tira uma de cada
// vez para mostrar qual ataque passa sem ela.
//
// Todo texto hostil é um marcador [LAB]; o CPF é o de exemplo das
// documentações, e o destino de fora é um host .test.
package pipeline
