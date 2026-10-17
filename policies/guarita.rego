# Decisão da Guarita para cada chamada de ferramenta. Nega por
# padrão; permite ou pede aprovação só para o que grants.rego concede
# ao agente naquele servidor; regras sobre argumentos negam mesmo o
# que foi concedido.
package guarita

import data.guarita.grants.agents

default decision := "deny"

decision := granted if {
	not denied
	granted := adjusted
	granted in {"allow", "require_approval"}
}

# adjusted rebaixa uma concessão "allow" a pedir aprovação quando a
# chamada sai do que é rotineiro.
default adjusted := "deny"

adjusted := "require_approval" if {
	grant == "allow"
	outside_lab
} else := grant

# outside_lab: e-mail para fora de lab.test. endswith com a arroba
# evita que "lab.test.example" passe por "lab.test".
outside_lab if {
	input.tool == "send_email"
	not endswith(lower(input.arguments.to), "@lab.test")
}

# grant é o que a tabela concede; indefinido se não concede nada.
grant := agents[input.agent][input.server][input.tool]

denied if count(reasons) > 0

# reasons junta os motivos para negar uma chamada concedida.
reasons contains "caminho absoluto" if {
	startswith(input.arguments.path, "/")
}

reasons contains "e-mail sem destinatário" if {
	input.tool == "send_email"
	not is_string(object.get(input.arguments, "to", null))
}

reasons contains "caminho sai do diretório" if {
	some part in split(input.arguments.path, "/")
	part == ".."
}
