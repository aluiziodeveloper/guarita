# Concessões por agente e servidor: agents[agente][servidor][ferramenta]
# é a decisão. O que não está aqui é negado. Cada agente recebe só o
# que a tarefa dele exige.
package guarita.grants

agents := {
	"relatorios": {"files": {
		"read_file": "allow",
		"list_dir": "allow",
	}},
	"manutencao": {
		"files": {
			"read_file": "allow",
			"write_file": "require_approval",
		},
		"db": {
			"delete_database": "require_approval",
			"purge_records": "require_approval",
		},
	},
}
