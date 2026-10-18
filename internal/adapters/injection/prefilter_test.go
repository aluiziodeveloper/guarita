package injection

import "testing"

// #region injection-prefilter-fuzz
// FuzzPrefilter confere, para textos que o fuzzer inventa, que o
// descarte por strings.Contains nunca muda a resposta de uma forma:
// match e a expressão sozinha concordam sempre.
func FuzzPrefilter(f *testing.F) {
	for _, seed := range []string{
		"", "Vendas estáveis no trimestre.", "[INST] [LAB] [/INST]",
		"<|im_start|>system", `<p style="display:none">`, "System: x",
		"<system>", "## instructions", "<!-- [LAB] nota -->",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		for _, group := range patterns {
			for _, form := range group.forms {
				if form.match(text) != form.re.MatchString(text) {
					t.Fatalf("%s: %q", group.signal, text)
				}
			}
		}
	})
}

// #endregion
