# Scion Docs Agent

A lightweight Q&A service that answers questions about Scion using Gemini CLI. Deployed as a standalone Cloud Run service.

## Endpoints

| Method | Path       | Description                                      |
|--------|------------|--------------------------------------------------|
| POST   | `/ask`     | Submit a query, returns a Markdown answer as JSON |
| GET    | `/chat`    | Embeddable chat widget (HTML)                     |
| POST   | `/refresh` | Triggers `git pull` to update bundled source      |
| GET    | `/health`  | Health check                                      |

## Local Development

Prerequisites: Go 1.25+, Gemini CLI (`npm install -g @google/gemini-cli`), `GEMINI_API_KEY` env var.

```bash
# Run the server
cd extras/docs-agent
go run .

# Test the /ask endpoint
curl -X POST http://localhost:8080/ask \
  -H "Content-Type: application/json" \
  -d '{"query": "What is Scion?"}'

# Open the chat widget
open http://localhost:8080/chat
```

Environment variables:
- `PORT` - Listen port (default: `8080`)
- `GEMINI_API_KEY` - Gemini API key (required)
- `GEMINI_SYSTEM_MD` - Path to system prompt (default: `$DOCS_AGENT_REPO_DIR/extras/docs-agent/system-prompt.md`)
- `DOCS_AGENT_TIMEOUT` - Query timeout in seconds (default: `60`)
- `DOCS_AGENT_WORKSPACE` - Workspace root directory (default: `/workspace`)
- `DOCS_AGENT_REPO_DIR` - Path to Scion source checkout (default: `$DOCS_AGENT_WORKSPACE/scion`)
- `DOCS_AGENT_MODEL` - Gemini model to use (default: `gemini-3.1-flash-lite-preview`)

## Testing

```bash
cd extras/docs-agent
go test ./...
```

## Docker Build

```bash
docker build -t docs-agent extras/docs-agent/
```

## Deploy to Cloud Run

```bash
cd extras/docs-agent
./deploy.sh
```

Override defaults with environment variables: `PROJECT_ID`, `REGION`, `SERVICE_NAME`.

## Embedding the Chat Widget

Add an iframe to your site pointing to the deployed service:

```html
<iframe src="https://your-service-url.run.app/chat"
        style="width: 100%; height: 500px; border: none;"></iframe>
```
