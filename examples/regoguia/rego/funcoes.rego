# Funções próprias, funções embutidas e erros estritos.
package guia.funcoes

# Uma função tem argumentos; com corpo verdadeiro, vale true.
internal(address) if endswith(lower(address), "@lab.test")

# object.get dá um valor padrão quando a chave não existe.
recipient := object.get(input.arguments, "to", "")

default decision := "require_approval"

decision := "allow" if internal(recipient)

message := sprintf("para %q: %s", [recipient, decision])
