# O not sobre o indefinido e o conflito entre regras completas.
package guia.armadilhas

# denied só vale com o booleano true; qualquer outra coisa é indefinida.
denied if input.blocked == true

decision := "allow" if not denied

# Duas regras completas com valores diferentes para a mesma entrada.
conflict := "allow" if input.tool == "read_file"

conflict := "deny" if startswith(input.tool, "read")
