package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/aluiziodeveloper/guarita/internal/adapters/policyopa"
	"github.com/aluiziodeveloper/guarita/internal/adapters/policystatic"
	"github.com/aluiziodeveloper/guarita/internal/app"
)

// errPolicyAndAllow recusa as duas fontes de política juntas.
var errPolicyAndAllow = errors.New("use -policy or -allow, not both")

// policyFlags declara as duas fontes de política. -allow volta como
// texto e é separado depois de Parse.
func policyFlags(flags *flag.FlagSet, c *config) (allow *string) {
	flags.StringVar(&c.policyDir, "policy", "",
		"diretório com as políticas em Rego")
	return flags.String("allow", "", "ferramentas permitidas")
}

// #region cmd-guarita-new-policy
// newPolicy escolhe o adaptador da porta PolicyEvaluator: as regras em
// Rego de -policy ou a lista de -allow. As duas juntas são erro, porque
// não se saberia qual decide. Uma política que não compila impede a
// partida.
func newPolicy(ctx context.Context, c config) (
	app.PolicyEvaluator, error,
) {
	if c.policyDir == "" {
		return policystatic.New(allowAll(c.allow))
	}
	if len(c.allow) > 0 {
		return nil, errors.Join(errUsage, errPolicyAndAllow)
	}
	return policyopa.Load(ctx, os.DirFS(c.policyDir))
}

// #endregion
