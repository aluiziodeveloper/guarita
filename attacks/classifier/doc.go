// Package classifier mede o classificador do Capítulo 15 contra o
// conjunto de referência de testdata: desenvolvimento, teste e
// ofuscação, com rótulos dados antes da medida. A medida exige um
// Ollama local e só roda com GUARITA_OLLAMA definido; o teste do
// modelo fora do ar roda sempre.
//
// Todo texto hostil do conjunto pede só um marcador [LAB] ou a
// ferramenta lab_noop, que não faz nada.
package classifier
