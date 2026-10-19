# Decisão da Guarita para cada chamada de ferramenta. Nega por
# padrão; permite ou pede aprovação só para o que grants.rego concede
# ao agente naquele servidor; regras sobre argumentos negam mesmo o
# que foi concedido.
package guarita

import data.guarita.grants.agents

default decision := "deny"

decision := granted if {
	not denied
	granted := grant
	granted in {"allow", "require_approval"}
}

# grant é o que a tabela concede; indefinido se não concede nada.
grant := agents[input.agent][input.server][input.tool]

denied if count(reasons) > 0

# reasons junta os motivos para negar uma chamada concedida.
reasons contains "caminho absoluto" if {
	startswith(input.arguments.path, "/")
}

reasons contains "caminho sai do diretório" if {
	some part in split(input.arguments.path, "/")
	part == ".."
}
