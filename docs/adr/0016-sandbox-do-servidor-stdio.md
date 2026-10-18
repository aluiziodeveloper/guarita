# ADR 0016: Sandbox do servidor stdio num contêiner, pelo docker run

- **Estado:** aceita
- **Data:** 2026-10-18
- **Contexto:** Capítulo 17; fuga da sandbox e servidores locais que fazem mais do que deviam

## Contexto

No modo stdio, a Guarita inicia o servidor MCP como subprocesso, com o usuário, a rede e o sistema de arquivos dela. Um servidor de arquivos que confere o caminho pelo prefixo recusa o `..` escrito e segue um *symlink* para fora da raiz (`examples/naivefiles`); no laboratório, os dois *symlinks*, o absoluto e o relativo, entregaram o segredo ao lado da raiz. Os inspetores da Parte IV olham as mensagens, e não o que o servidor faz na máquina.

## Decisão

1. Com `-sandbox imagem`, o comando depois de `--` roda num `docker run` montado pelo pacote `internal/adapters/sandbox`, com `os/exec`: `--init`, `--network none`, `--read-only` com `/tmp` em tmpfs sem execução, usuário 65532, `--cap-drop ALL`, `no-new-privileges`, `--pids-limit 64` e `--memory 256m`. O perfil seccomp padrão do Docker continua valendo, porque nenhuma opção o troca. O ambiente do host não entra.
2. O perfil recusa usuário root, imagem que começa com hífen, limites ausentes e montagens com vírgula, que mudariam o sentido da opção `--mount`.
3. Cada `-sandbox-mount` é só de leitura, a menos de `:rw`. A origem é resolvida com `filepath.EvalSymlinks`, e o caminho real não pode ficar em diretórios do sistema ou de credenciais da home, nem contê-los, nem conter a home. A comparação é por componente, com `filepath.Rel`.
4. O cancelamento manda SIGTERM ao cliente do Docker, e não o SIGKILL padrão de `exec.CommandContext`, e o contêiner é removido pelo nome no fim. No laboratório, sem `--init`, o contêiner ignorou o SIGTERM repassado ao PID 1, e com SIGKILL no cliente ficou de pé até a remoção.
5. O servidor de arquivos do laboratório usa `os.Root`, que recusa o *symlink* para fora da raiz; ele é o exemplo de como um servidor deve resolver caminhos, e a sandbox não depende dele.
6. Os testes de integração, em `attacks/sandbox`, rodam com o Docker do *runner* da CI e são pulados sem Docker.

## Alternativas consideradas

1. **O SDK do Docker em Go.** Rejeitada: traz uma árvore grande de dependências para montar uma linha de comando que o cliente já valida, e a Guarita passaria a falar com o *socket* do Docker diretamente. O `docker run` também é o que o operador consegue reproduzir à mão.
2. **Isolar no próprio processo, com namespaces, seccomp e Landlock.** Adiada: exige código específico do Linux, privilégios para criar os namespaces e um perfil próprio, que o Docker já entrega testado.
3. **Rede com lista de destinos.** Adiada: um servidor que precise de rede pede um proxy de saída com lista de hosts, que não cabe neste capítulo. Sem rede é o padrão seguro, e o servidor que não funciona assim não roda na sandbox.
4. **Montar a home e confiar no servidor.** Rejeitada: é a configuração que entrega `.ssh` e `.aws` ao primeiro servidor comprometido.

## Consequências

- Na sonda do laboratório, o perfil negou as cinco tentativas: escrever na raiz, conectar ao host, ler um arquivo do host, usar o chaveiro do kernel e virar root. O contêiner com as opções padrão do Docker rodou como root, escreveu na raiz, alcançou o host e conseguiu `setuid(0)`; o chaveiro foi negado nele pelo seccomp padrão.
- Na fuga por *symlink*, só o servidor ingênuo no host entregou o segredo; na sandbox, o mesmo código ingênuo não o alcançou, porque o contêiner não vê o caminho.
- O kernel do host é compartilhado: uma falha nele atravessa o contêiner. A imagem não é verificada (Capítulo 20).
- O modo HTTP não tem sandbox: os servidores são remotos.
- Entre a conferência da montagem e o `docker run`, um componente do caminho pode ser trocado; a janela é curta, e quem pode trocá-lo já escreve no diretório montado.
