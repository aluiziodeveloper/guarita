# Política mínima dos testes do adaptador: lê a ferramenta e um
# argumento, e nega o resto por padrão.
package guarita

default decision := "deny"

decision := "allow" if input.tool == "read_file"

decision := "require_approval" if {
	input.tool == "write_file"
	input.arguments.path != "notes.txt"
}
