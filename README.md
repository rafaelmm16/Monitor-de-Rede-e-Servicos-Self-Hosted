# NetPulse — Monitor de Rede e Serviços Self-Hosted (Go + Next.js)

Um monitor de infraestrutura e serviços self-hosted (estilo Uptime Kuma simplificado), projetado com **daemon em Go compilado em binário único** (`CGO_ENABLED=0`) e **dashboard em tempo real com Next.js + Tailwind CSS**, orquestrado via **Docker Compose**.

## Arquitetura

- **Backend (`backend/`)**: Daemon leve em Go com SQLite embarcado em Go puro (`modernc.org/sqlite`), sem dependências externas ou CGO.
  - **Health Checks**:
    - `HTTP / HTTPS`: status code esperado, tempo de resposta (ms), busca opcional de palavra-chave no body e inspeção automática de certificado TLS.
    - `Ping (ICMP)`: ICMP Echo Request/Reply com fallback para socket UDP não-privilegiado.
    - `TCP`: verificação de abertura de porta TCP (`host:port`) e latência de handshake.
    - `SSL`: verificação dedicada de validade de certificado TLS e contagem regressiva para expiração (status `degraded` configurável).
  - **API REST + WebSocket**: atualizações em tempo real (`/api/ws`) para o dashboard.
  - **Alertas via Webhook**: suporte nativo a **Discord** (Embeds coloridos), **Telegram** (Bot API) e **Generic JSON Webhook**, disparados automaticamente em transições de status (`UP` ↔ `DOWN` / `DEGRADED`), com registro de **Incidentes** (início, fim e duração).
- **Frontend (`frontend/`)**: Dashboard em Next.js (App Router) + Tailwind CSS v4.
  - KPIs em tempo real (Total, Operational, Down/Degraded, Latência Média).
  - Barra de heartbeat dos últimos 30 checks, gráfico SVG interativo de latência e histórico de incidentes.
  - Modais para gerenciamento de monitores e configuração/teste de webhooks.

## Subindo com Docker Compose

```bash
docker compose up --build -d
```

Para subir também **Prometheus** (`:9090`) e **Grafana** (`:3001`) pré-configurados:

```bash
docker compose --profile observability up --build -d
```

- **Dashboard**: `http://localhost:3000`
- **API / WebSocket**: `http://localhost:8080`

> O container `backend` inclui `cap_add: [NET_RAW]` para permitir checks ICMP (Ping) nativos dentro do Docker.

## Compilação em Binário Único (Backend Go)

Para compilar o daemon em um único binário estático sem CGO:

```bash
cd backend
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/netpulse ./cmd/server
```

Para executar localmente:

```bash
./bin/netpulse
```

## Desenvolvimento Local

### Backend
```bash
cd backend
go test ./...
go run ./cmd/server
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

## Endpoints Principais da API

| Método | Rota | Descrição |
| :--- | :--- | :--- |
| `GET` | `/api/health` | Status do daemon |
| `GET` | `/api/status` | Dados para a Status Page pública (`/status`) |
| `GET` | `/api/badge/{id}` | Badge SVG dinâmico para embutir em READMEs/docs |
| `GET` | `/metrics` | Métricas no formato Prometheus / OpenMetrics |
| `GET` | `/api/ws` | Stream WebSocket em tempo real |
| `GET` | `/api/monitors` | Lista monitores e estatísticas 24h |
| `POST` | `/api/monitors` | Cria um novo monitor (suporta `max_retries` e `maintenance`) |
| `PUT` | `/api/monitors/{id}` | Atualiza um monitor |
| `DELETE` | `/api/monitors/{id}` | Remove um monitor |
| `POST` | `/api/monitors/{id}/check` | Executa check imediato |
| `GET` | `/api/monitors/{id}/checks` | Histórico de latência e status |
| `GET` | `/api/incidents` | Histórico de quedas/incidentes |
| `GET` / `POST` | `/api/webhooks` | Lista ou cria canais de alerta |
| `POST` | `/api/webhooks/{id}/test` | Envia alerta de teste |
| `GET` / `POST` | `/api/config/export` / `import` | Backup e restauração em JSON |

> **Autenticação opcional:** Defina `NETPULSE_AUTH_TOKEN` no ambiente do backend para proteger rotas de escrita, webhooks e backup com `Authorization: Bearer <token>`.
