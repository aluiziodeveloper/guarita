package approvalforgery

// attempt é uma tentativa de aprovar e o estado HTTP esperado.
type attempt struct {
	name   string
	header map[string]string
	want   int
}

// #region attacks-forgery-attempts
// attempts vai da forja mais simples à repetição de uma aprovação
// legítima. A credencial da ana aparece onde o navegador dela a
// mandaria sozinho.
func attempts(ana string) []attempt {
	return []attempt{
		{"sem credencial", nil, 401},
		{"token do agente", map[string]string{
			"Authorization": "Bearer eyJhbGciOiJFZERTQSJ9.e30.x",
		}, 401},
		{"página de outra origem", map[string]string{
			"Authorization":  ana,
			"Origin":         "http://evil.lab.test",
			"Sec-Fetch-Site": "cross-site",
		}, 403},
		{"rebinding de DNS", map[string]string{
			"Authorization": ana, "Host": "evil.lab.test",
		}, 403},
		{"a ana aprova", map[string]string{"Authorization": ana}, 204},
		{"repetição", map[string]string{"Authorization": ana}, 409},
	}
}

// #endregion
