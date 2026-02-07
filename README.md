# Idea Bot

A lightweight Telegram bot for collecting and analyzing feature ideas using Claude AI.

## Features

- 📥 Collect ideas via `/idea` command in Telegram groups
- 🤖 Automatic AI-powered analysis using Claude (Anthropic)
- 📊 Structured output: category, priority, complexity, affected components
- 🌐 Web UI for viewing and managing ideas
- 💾 SQLite storage
- 🔄 Duplicate detection
- ⚡ Rate limiting

## Quick Start

### 1. Create a Telegram Bot

1. Message [@BotFather](https://t.me/BotFather) on Telegram
2. Create a new bot: `/newbot`
3. Save the token

### 2. Get Anthropic API Key

1. Sign up at [console.anthropic.com](https://console.anthropic.com)
2. Create an API key in Settings → API Keys

### 3. Configure Environment

```bash
cp .env.example .env
# Edit .env with your keys
```

### 4. Run

```bash
docker compose up -d
```

## Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `TELEGRAM_BOT_TOKEN` | Token from @BotFather | ✅ |
| `TELEGRAM_ALLOWED_GROUPS` | Allowed group IDs (comma-separated) | ❌ |
| `ANTHROPIC_API_KEY` | Anthropic API key | ✅ |
| `CLAUDE_MODEL` | Claude model (default: claude-sonnet-4-20250514) | ❌ |
| `SYSTEM_PROMPT_FILE` | Path to custom system prompt file | ❌ |
| `WEB_PORT` | Web interface port (default: 8080) | ❌ |
| `WEB_BASE_URL` | Base URL for idea links (default: http://localhost:8080) | ❌ |
| `WEB_USERNAME` | Web interface login | ✅ |
| `WEB_PASSWORD` | Web interface password | ✅ |
| `SQLITE_PATH` | Database path (default: /data/ideas.db) | ❌ |
| `RATE_LIMIT_PER_USER` | Ideas per user per hour (default: 5) | ❌ |
| `RATE_LIMIT_GLOBAL` | Global ideas per hour (default: 50) | ❌ |

### Getting Group ID

1. Add the bot to a group
2. Send any message
3. Open `https://api.telegram.org/bot<TOKEN>/getUpdates`
4. Find `chat.id` (negative number)

### Custom System Prompt

You can customize the AI analysis by providing your own system prompt:

1. Create a file with your prompt (e.g., `prompts/my-project.txt`)
2. Set `SYSTEM_PROMPT_FILE=/app/prompts/my-project.txt`
3. Mount the file in Docker:

```yaml
volumes:
  - ./prompts:/app/prompts:ro
```

Example custom prompt:

```text
You are an AI assistant analyzing feature ideas for MyApp - a task management application.

## Project Context
- Backend: Node.js + Express
- Frontend: React
- Database: PostgreSQL
- Mobile: React Native

## Your Task
Analyze submitted ideas and provide structured output for planning.
Always respond in English.
Return ONLY valid JSON without markdown.
```

## Usage

### Telegram

```
/idea add dark mode toggle to settings
```

The bot will analyze the idea and respond with:
- Title and summary
- Category (feature/improvement/bug/integration)
- Priority (low/medium/high/critical)
- Complexity (trivial/small/medium/large/epic)
- User Story and acceptance criteria
- Technical notes

### Web UI

Open `http://your-server:8080` (or configured domain).

- View ideas list with filters
- View idea details
- Change status
- Add admin notes
- Delete ideas

## Deployment

### With Docker Compose

```bash
docker compose up -d
```

### With Caddy (HTTPS)

Add to Caddyfile:

```
ideas.example.com {
    reverse_proxy localhost:8080
}
```

Reload Caddy: `systemctl reload caddy`

### Monitoring

```bash
# Logs
docker logs -f idea-bot

# Status
docker ps

# Health check
curl http://localhost:8080/health
```

## Development

```bash
# Local run
go run ./cmd/bot

# Build
go build -o idea-bot ./cmd/bot

# Tests
go test ./...
```

## Project Structure

```
├── cmd/bot/main.go           # Entry point
├── internal/
│   ├── config/               # Configuration (Viper)
│   ├── domain/
│   │   ├── model/            # Data models
│   │   └── service/          # Business logic
│   ├── storage/              # SQLite repository
│   ├── telegram/             # Telegram bot
│   └── web/                  # HTTP handlers + templates
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

## License

MIT
