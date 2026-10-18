package domain

import "testing"

func TestStricter(t *testing.T) {
	allow, approve, deny := DecisionAllow, DecisionRequireApproval,
		DecisionDeny
	for _, c := range []struct{ a, b, want Decision }{
		{allow, allow, allow},
		{allow, approve, approve},
		{approve, allow, approve},
		{approve, deny, deny},
		{deny, allow, deny},
		{allow, "execute", deny},
		{"", allow, deny},
	} {
		if got := Stricter(c.a, c.b); got != c.want {
			t.Errorf("Stricter(%q, %q) = %q; want %q", c.a, c.b, got,
				c.want)
		}
	}
}
