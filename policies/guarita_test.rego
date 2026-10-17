# Testes das regras com opa test. "with" troca o input e, quando
# preciso, a tabela de concessões, sem mexer em grants.rego.
package guarita_test

import data.guarita

# Uma tabela própria dos testes: as regras são testadas sem depender
# das concessões que o operador mantém.
fake := {"a": {"s": {"t": "allow", "u": "require_approval"}}}

test_nega_por_padrao if {
	guarita.decision == "deny" with input as {}
}

test_concede_o_que_a_tabela_da if {
	guarita.decision == "allow" with input as {
		"agent": "a", "server": "s", "tool": "t", "arguments": {},
	}
		with data.guarita.grants.agents as fake
}

test_aprovacao_vem_da_tabela if {
	guarita.decision == "require_approval" with input as {
		"agent": "a", "server": "s", "tool": "u", "arguments": {},
	}
		with data.guarita.grants.agents as fake
}

test_concessao_e_por_servidor if {
	guarita.decision == "deny" with input as {
		"agent": "a", "server": "outro", "tool": "t", "arguments": {},
	}
		with data.guarita.grants.agents as fake
}

test_valor_fora_do_conjunto_nega if {
	guarita.decision == "deny" with input as {
		"agent": "a", "server": "s", "tool": "t", "arguments": {},
	}
		with data.guarita.grants.agents as {"a": {"s": {"t": "sim"}}}
}

test_argumento_nega_o_concedido if {
	guarita.decision == "deny" with input as {
		"agent": "a", "server": "s", "tool": "t",
		"arguments": {"path": "docs/../../segredo"},
	}
		with data.guarita.grants.agents as fake
	guarita.reasons == {"caminho sai do diretório"} with input as {
		"arguments": {"path": "docs/../../segredo"},
	}
}

test_relatorios_nao_apaga_o_banco if {
	guarita.decision == "deny" with input as {
		"agent": "relatorios", "server": "db",
		"tool": "delete_database", "arguments": {},
	}
}
