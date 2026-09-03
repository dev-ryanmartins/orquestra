# Orquestra

Microserviço de fila de mensagens concorrente, escrito em Go com a biblioteca padrão.

O Orquestra recebe tarefas por HTTP, coloca cada tarefa em uma fila limitada e distribui o trabalho entre um conjunto de workers. O status fica disponível para consulta durante todo o ciclo de vida da tarefa:

```text
pending → processing → completed
                    ↘ failed
```

O projeto foi pensado como uma base pequena, legível e extensível para serviços assíncronos. O processador padrão simula o trabalho com um pequeno atraso configurável e registra a conclusão; em um cenário real, ele pode ser substituído pelo handler que executa o domínio da aplicação.

## Requisitos

- Go 1.25 ou superior
- Nenhuma dependência externa

## Como executar

```bash
go run ./cmd/orquestra
```

Por padrão, o servidor escuta em `http://localhost:8080`. A rota `/api` é aceita para funcionar diretamente atrás de um proxy; as mesmas rotas também podem ser chamadas sem o prefixo.

### Configuração

Todas as configurações são opcionais:

| Variável | Padrão | Descrição |
| --- | ---: | --- |
| `PORT` | `8080` | Porta HTTP do serviço |
| `ORQUESTRA_WORKERS` | `4` | Quantidade de goroutines consumidoras |
| `ORQUESTRA_QUEUE_CAPACITY` | `128` | Quantidade máxima de tarefas aguardando |
| `ORQUESTRA_PROCESSING_DELAY_MS` | `100` | Atraso do processador de demonstração |

Exemplo com uma fila pequena para observar a concorrência:

```bash
ORQUESTRA_WORKERS=3 \
ORQUESTRA_QUEUE_CAPACITY=20 \
ORQUESTRA_PROCESSING_DELAY_MS=250 \
go run ./cmd/orquestra
```

## API

### Verificar saúde

```bash
curl -i http://localhost:8080/api/healthz
```

Resposta:

```json
{"status":"ok"}
```

### Enfileirar uma tarefa

```bash
curl -i -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "gerar-relatorio",
    "payload": {
      "cliente": "acme",
      "formato": "pdf"
    }
  }'
```

O serviço responde `202 Accepted` imediatamente, sem esperar o processamento terminar:

```json
{
  "id": "8c3f...",
  "name": "gerar-relatorio",
  "payload": {"cliente":"acme","formato":"pdf"},
  "status": "pending",
  "created_at": "2026-09-03T12:00:00Z"
}
```

Se a fila estiver cheia, a resposta será `503 Service Unavailable`. Isso aplica backpressure em vez de consumir memória indefinidamente.

### Consultar uma tarefa

Substitua o ID pelo valor retornado no POST:

```bash
curl -s http://localhost:8080/api/tasks/8c3f... | jq
```

### Consultar estatísticas

```bash
curl -s http://localhost:8080/api/stats | jq
```

A resposta informa o total por status e o uso atual da fila:

```json
{
  "total": 12,
  "pending": 3,
  "processing": 4,
  "completed": 5,
  "failed": 0,
  "queue_depth": 3,
  "queue_capacity": 128
}
```

## Testando a concorrência

Com o servidor rodando em um terminal, abra outro e envie várias tarefas ao mesmo tempo:

```bash
seq 1 20 | xargs -P 8 -I {} curl -s -X POST http://localhost:8080/api/tasks \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"tarefa-{}\",\"payload\":{\"numero\":{}}}" \
  -o /dev/null -w "tarefa {} -> %{http_code}\n"
```

Observe o processamento em lotes no terminal do servidor e acompanhe os números:

```bash
watch -n 0.2 'curl -s http://localhost:8080/api/stats'
```

Com quatro workers e um atraso de 250 ms, quatro tarefas podem ser processadas simultaneamente. A fila é protegida por um channel buffered e o mapa de status por `sync.RWMutex`.

## Testes e qualidade

Execute os testes unitários e de integração:

```bash
go test ./...
```

Execute o detector de condições de corrida — este é o teste recomendado para a parte concorrente:

```bash
go test -race ./...
```

Verifique problemas comuns de implementação:

```bash
go vet ./...
```

Compile um binário:

```bash
go build -o bin/orquestra ./cmd/orquestra
./bin/orquestra
```

## Estrutura

```text
.
├── cmd/orquestra/          # Entrada do processo e configuração
├── internal/httpapi/       # Rotas HTTP, validação e respostas
├── internal/processor/     # Processador padrão substituível
├── internal/queue/         # Channel buffered e ciclo de vida dos workers
├── internal/service/       # Orquestração entre fila e armazenamento
├── internal/store/         # Estado em memória protegido por mutex
├── internal/task/          # Modelo, status e geração de IDs
├── go.mod
└── README.md
```

## Decisões técnicas

- **Biblioteca padrão:** HTTP com `net/http`, logs com `log/slog`, IDs com `crypto/rand` e sincronização com `sync`.
- **Backpressure explícito:** a fila possui capacidade limitada e rejeita novas tarefas quando está cheia.
- **Shutdown seguro:** o servidor para de aceitar tráfego, fecha a fila e espera os workers terminarem as tarefas já enfileiradas.
- **Estado em memória:** mantém o exemplo autocontido e fácil de executar. Para produção, o store pode ser trocado por uma persistência durável sem alterar a API HTTP.
- **Processador injetável:** a fila não conhece a regra de negócio. O serviço recebe um `Processor`, o que torna o núcleo testável e permite conectar handlers reais.