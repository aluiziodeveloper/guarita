# Regras completas, valor padrão e indefinido.
package guia.regras

default decision := "deny"

decision := "allow" if input.tool == "read_file"

decision := "require_approval" if {
	input.tool == "delete_database"
	input.agent == "manutencao"
}

# owner é indefinida quando a entrada não tem servidor.
owner := input.server
