// Package abuse reproduz o abuso de recursos do Capítulo 18: um agente
// em laço chama uma ferramenta paga até esgotar a fatura, e um servidor
// que falha devagar segura cada chamada do agente. O servidor de
// laboratório só conta as chamadas; nenhuma cobrança é real.
package abuse
