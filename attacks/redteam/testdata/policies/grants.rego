package guarita.grants

# As concessões do laboratório do Capítulo 22. O agente de relatórios
# lê arquivos e documentos, pesquisa na web e faz pedidos HTTP pelos
# servidores do laboratório, e nada mais; o de manutenção pode apagar
# o banco, com a aprovação de uma pessoa.
agents := {
	"relatorios": {
		"files": {"read_file": "allow"},
		"notes": {"read_file": "allow", "Read-File": "allow"},
		"docs": {"read_doc": "allow", "search_docs": "allow"},
		"web": {"web_search": "allow"},
		"mail": {"http_request": "allow"},
		"db": {"read_file": "allow"},
		"status": {"check_status": "allow"},
	},
	"manutencao": {"db": {"delete_database": "require_approval"}},
}
