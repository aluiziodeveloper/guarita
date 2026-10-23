# Regras de conjunto e de objeto, iteração e compreensões.
package guia.conjuntos

grants := {
	"files": ["read_file", "list_dir"],
	"db": ["delete_database"],
}

# Cada corpo verdadeiro acrescenta um elemento ao conjunto.
reasons contains "caminho absoluto" if startswith(input.path, "/")

reasons contains "caminho sai do diretório" if {
	some part in split(input.path, "/")
	part == ".."
}

# Uma chave por servidor, com o número de ferramentas concedidas.
tools_per_server[server] := count(tools) if {
	some server, tools in grants
}

# every vale se todos os elementos cumprem o corpo.
all_known if {
	every tool in input.tools {
		tool in all_tools
	}
}

# A compreensão monta o conjunto das ferramentas de todos os servidores.
all_tools := {tool | some tools in grants; some tool in tools}
