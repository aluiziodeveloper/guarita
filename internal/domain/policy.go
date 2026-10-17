package domain

// #region domain-policy-input
// PolicyInput é o contrato entre quem aplica a decisão e quem decide:
// quem pede, em que servidor e o quê. A política não lê o pedido HTTP
// nem a mensagem MCP; só isto.
//
// Agent vem da identidade verificada (Capítulo 8) e fica vazio sem
// ela, como no stdio. Server é o namespace dado pelo operador
// (Capítulo 10), vazio para o servidor de -upstream.
type PolicyInput struct {
	Agent  string
	Server string
	Call   Call
}

// #endregion
