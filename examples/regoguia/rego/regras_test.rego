# Testes com opa test: regras test_ que passam quando valem.
package guia.regras_test

import data.guia.regras

test_nega_por_padrao if regras.decision == "deny" with input as {}

test_permite_a_leitura if {
	regras.decision == "allow" with input as {"tool": "read_file"}
}

test_manutencao_pede_aprovacao if {
	regras.decision == "require_approval" with input as {
		"agent": "manutencao", "tool": "delete_database",
	}
}

test_relatorios_nao_apaga if {
	regras.decision == "deny" with input as {
		"agent": "relatorios", "tool": "delete_database",
	}
}

# not inverte o indefinido: sem servidor, owner não existe.
test_dono_indefinido_sem_servidor if not regras.owner with input as {}
