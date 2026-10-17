# Árvores de ataque — v1

Cada árvore parte do objetivo do atacante na raiz. Um nó **OU** é atingido quando qualquer filho é atingido; um nó **E** exige todos os filhos. As folhas marcadas com o capítulo são os pontos em que a Guarita vai pôr uma defesa.

## `tool-poisoning` — fazer o agente ler `~/.ssh/id_rsa`

```text
OBJETIVO: o agente entrega o conteúdo de ~/.ssh/id_rsa ao atacante
└── E
    ├── 1. A instrução hostil chega ao modelo
    │   └── OU
    │       ├── 1.1 Descrição envenenada anunciada desde o início
    │       │       defesa: varredura estrutural antes do pin (Cap. 10,
    │       │       parcial: pega o texto disfarçado, não o visível)
    │       ├── 1.2 Descrição legítima trocada depois da aprovação
    │       │       defesa: pin por hash (Cap. 9)
    │       └── 1.3 Esquema com parâmetro extra para o conteúdo
    │               defesa: pin da definição inteira (Cap. 9)
    ├── 2. O modelo propõe a chamada com o caminho do segredo
    │   └── OU
    │       ├── 2.1 Redirecionamento plausível (“o arquivo mudou”)
    │       │       medido no Cap. 1: 20 de 20 com qwen3:1.7b
    │       └── 2.2 Ordem explícita de ignorar o usuário
    │               medido no Cap. 1: 0 de 20 com qwen3:1.7b
    ├── 3. A chamada atravessa a Guarita
    │   └── E
    │       ├── 3.1 Nenhuma política nega leitura fora do escopo
    │       │       defesa: políticas por ferramenta (Cap. 11)
    │       └── 3.2 Nenhum inspetor barra o caminho nos argumentos
    │               defesa: DLP e lista de destinos (Cap. 13)
    └── 4. O servidor lê o arquivo do host
        └── OU
            ├── 4.1 O servidor tem acesso ao diretório pessoal
            │       defesa: sandbox sem montagem do host (Cap. 17)
            └── 4.2 Um symlink leva para fora do escopo
                    defesa: resolução segura de caminhos (Cap. 17)
```

A raiz é um nó **E**: basta cortar um dos quatro ramos para o ataque falhar. Por isso a Guarita defende em camadas, e nenhuma delas precisa ser perfeita sozinha.

## Resumo das demais ameaças

| ID | Raiz | Ramos principais | Onde cortar |
|---|---|---|---|
| `rug-pull` | Usar uma ferramenta aprovada com outro comportamento | trocar descrição OU esquema OU implementação | hash da definição (Cap. 9); sandbox para a implementação (Cap. 17) |
| `tool-shadowing` | Fazer o agente chamar a ferramenta errada | nome repetido E escolha pela descrição | namespace por servidor e silhueta única no catálogo (Cap. 10) |
| `indirect-prompt-injection` | Ordenar o agente pela resposta de uma ferramenta | texto hostil na resposta E o modelo obedecer | inspeção e marcação da resposta (Cap. 14) |
| `excessive-agency` | Executar ação destrutiva permitida por engano | ferramenta perigosa exposta E nenhuma política restringe | menor privilégio (Cap. 11) |
| `data-exfiltration` | Levar dados sensíveis num argumento | dado sensível no contexto E argumento sem inspeção | DLP nos argumentos (Cap. 13) |
| `sandbox-escape` | Ler ou alterar o host a partir do servidor | rede aberta OU arquivos montados OU symlink | contêiner restrito (Cap. 17) |
| `credential-theft` | Usar a credencial da Guarita em nome do agente | *token* do usuário repassado OU credencial compartilhada | credencial por servidor, sem repasse (Cap. 8) |
| `supply-chain` | Rodar código novo sob uma versão aprovada | pacote atualizado sem verificação | versão fixa e assinatura (Cap. 20) |
| `resource-abuse` | Esgotar cota ou fatura | laço de chamadas E ausência de limite | cota por agente e ferramenta (Cap. 18) |
| `insecure-parsing` | Derrubar ou enganar o *parser* | mensagem grande OU ambígua OU malformada | limites e *fuzzing* (Cap. 6) |
| `approval-forgery` | Obter aprovação sem decisão humana real | aprovação forjada OU fadiga | máquina de estados e limites (Cap. 12) |
| `dns-rebinding` | Chamar ferramentas pelo navegador do usuário | nome do atacante resolvido para 127.0.0.1 E endpoint que aceita qualquer `Host` | lista de `Host` e de `Origin`, escuta em loopback (Cap. 7); autenticação (Cap. 8) |
