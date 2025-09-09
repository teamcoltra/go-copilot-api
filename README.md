# 🤖 Copilot OpenAI API (Go Edition)

**License:** See [LICENSE](LICENSE) — YO LICENSE, Version 1.0, February 2025  
Copyright (C) 2025 Travis Peacock


[![Go 1.21+](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/dl/)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](https://www.docker.com/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A high-performance Go proxy server that turns GitHub Copilot's chat completion/embeddings capabilities into an OpenAI-compatible API service, with Anthropic compatibility and robust security.


## ✨ Key Features

🚀 **Advanced Integration**
- Seamless GitHub Copilot chat completion API proxy (`/v1/chat/completions`)
- Embeddings API proxy (`/v1/embeddings`)
- Anthropic API compatibility (`/v1/messages`, experimental)
- Real-time streaming response support
- High-performance, concurrent request handling

🔐 **Security & Reliability**
- Secure authentication middleware (Bearer token)
- Automatic Copilot token management and refresh
- Built-in CORS support for web applications
- Clear error handling (401, 403, etc.)

💻 **Universal Compatibility**
- Cross-platform config auto-detection (Windows, Unix, macOS)
- Docker containerization ready
- Flexible deployment and configuration

---

## 🚀 Prerequisites

- Go 1.21+ (https://golang.org/dl/)
- GitHub Copilot subscription
- GitHub authentication token (see below)

---

## 📦 Installation

1. Clone the repository:
```bash
git clone https://github.com/your-org/copilot-openai-api-go.git
cd go-copilot-api
```

2. Build the binary:
```bash
go build -o bin/go-copilot-api ./cmd/go-copilot-api
```

---

## ⚙️ Configuration

Set up environment variables (or use a `.env` file):

| Variable                  | Description                                         | Default                |
|---------------------------|-----------------------------------------------------|------------------------|
| `COPILOT_TOKEN`           | Required. API access token for authentication.      | Randomly generated     |
| `COPILOT_OAUTH_TOKEN`     | Copilot OAuth token (auto-detected if not set)      | (auto)                 |
| `COPILOT_SERVER_PORT`     | Port to listen on (overrides SERVER_ADDR)           | `9191`                 |
| `SERVER_ADDR`             | Address to listen on (used if COPILOT_SERVER_PORT unset) | `:8080`           |
| `CORS_ALLOWED_ORIGINS`    | Comma-separated list of allowed CORS origins        | `*`                    |
| `DEBUG`                   | Enable debug logging                                | `false`                |
| `DEFAULT_MODEL`           | Default model to use if not specified in request    | *(none)*               |

**Copilot OAuth Token Auto-Detection:**
- If `COPILOT_OAUTH_TOKEN` is not set, the app will look for your Copilot config:
  - **Unix/macOS:** `~/.config/github-copilot/apps.json`
  - **Windows:** `%LOCALAPPDATA%/github-copilot/apps.json`
- The first available `oauth_token` will be used.

**How to get a valid Copilot configuration?**
- Install any official GitHub Copilot plugin (VS Code, JetBrains, Vim, etc.), sign in, and the config files will be created automatically.

---

## 🖥️ Local Run

Start the server:
```bash
go run ./cmd/go-copilot-api
```
or
```bash
bin/go-copilot-api
```

---

## 🐳 Docker Run

Build and run with Docker:
```bash
docker build -t copilot-openai-api-go .
docker run --rm -p 9191:9191 \
    -v ~/.config/github-copilot:/home/appuser/.config/github-copilot \
    -e COPILOT_TOKEN=your_access_token_here \
    copilot-openai-api-go
```
- On Windows, use `%LOCALAPPDATA%/github-copilot` for the volume mount.

---

## 🔄 Making API Requests

### Chat Completions
```bash
curl -X POST http://localhost:9191/v1/chat/completions \
  -H "Authorization: Bearer your_access_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [{"role": "user", "content": "Hello, Copilot!"}]
  }'
```

### Embeddings
```bash
curl -X POST http://localhost:9191/v1/embeddings \
  -H "Authorization: Bearer your_access_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "copilot-text-embedding-ada-002",
    "input": ["The quick brown fox", "Jumped over the lazy dog"]
  }'
```

### Anthropic API Compatibility (Experimental)
```bash
curl -X POST http://localhost:9191/v1/messages \
  -H "Authorization: Bearer your_access_token_here" \
  -H "Content-Type: application/json" \
  -d '{ ...Anthropic API message format... }'
```

### List Available Models

Fetch the list of available models and their capabilities:
```bash
curl -X GET http://localhost:9191/v1/models
```
- This endpoint does **not** require authentication.
- The models list is fetched from GitHub's model catalog API at server startup and periodically refreshed (every 6 hours).
- The response is a JSON array of model objects, including `id`, `name`, `summary`, and more.
- Use the `"id"` field (e.g., `"gpt-5-mini"`, `"gpt-4o-mini-2024-07-18"`) as the `"model"` value in your requests.

#### Example: Find the correct model name for GPT-5 Mini
```bash
curl -s http://localhost:9191/v1/models | jq '.[] | select(.id | test("5-mini"))'
```

---

### Default Model

You can set a default model for all requests by adding to your `.env` or environment:

```
DEFAULT_MODEL=gpt-5-mini
```

- If a client request does **not** specify a `"model"` field, this value will be used automatically for `/v1/chat/completions`, `/v1/embeddings`, and `/v1/messages`.
- If `DEFAULT_MODEL` is **not set**, and the client omits `"model"`, **no model is sent** to Copilot (Copilot will auto-select).
- If the client provides a `"model"`, that value is always used as-is.

#### Example `.env`:
```
COPILOT_TOKEN=your_token_here
DEFAULT_MODEL=gpt-5-mini
COPILOT_SERVER_PORT=9191
```

## 🔌 API Reference

### POST /v1/chat/completions
- Proxies requests to GitHub Copilot's Completions API.
- **Headers:** `Authorization: Bearer <your_access_token>`, `Content-Type: application/json`
- **Body:** Must include `"messages"`. You may include `"model"` (see `/v1/models` for valid values). If omitted and `DEFAULT_MODEL` is set, it will be injected.
- **Response:** Streams responses directly from Copilot (supports streaming and non-streaming).

### POST /v1/embeddings
- Proxies requests to Copilot's Embeddings API.
- **Headers:** `Authorization: Bearer <your_access_token>`, `Content-Type: application/json`
- **Body:** Must include `"input"`. You may include `"model"` (see `/v1/models`). If omitted and `DEFAULT_MODEL` is set, it will be injected.
- **Response:** JSON from Copilot's embeddings API.

### POST /v1/messages
- Converts Anthropic API format to Copilot chat completion format.
- **Headers:** `Authorization: Bearer <your_access_token>`, `Content-Type: application/json`
- **Body:** Anthropic-compatible. You may include `"model"` (see `/v1/models`). If omitted and `DEFAULT_MODEL` is set, it will be injected.
- **Response:** Anthropic API-compatible response.

### GET /v1/models
- Returns a list of available models and their capabilities.
- **No authentication required.**
- **Response:** JSON array of models as provided by GitHub's model catalog API.
- **Tip:** Use the `"id"` field as the `"model"` value in your requests.

---

## 🔒 Authentication

- Set `COPILOT_TOKEN` in your environment.
- Include in request headers:
  ```
  Authorization: Bearer your_access_token_here
  ```

---

## ⚠️ Error Handling

- 401: Missing/invalid authorization header
- 403: Invalid access token
- Other errors are propagated from GitHub Copilot API

---

## 🛡️ Security Best Practices

- Configure CORS for your specific domains (default: `*`)
- Safeguard your `COPILOT_TOKEN` and GitHub OAuth token
- Built-in token management with concurrent access protection

---

## 🧪 Experimental Features

- Anthropic API compatibility (`/v1/messages`)

---

## 🗂️ Project Structure

```
go-copilot-api/
├── cmd/
│   └── go-copilot-api/
│       └── main.go         # Application entrypoint
├── internal/
│   ├── api/                # HTTP handlers and routing
├── pkg/
│   └── config/             # Configuration loading
├── test/                   # Test files
├── LICENSE                 # YO LICENSE
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
└── README.md               # This file
```

---

## 🧪 Testing

Run all tests:
```sh
go test ./...
```

---

## 📄 License

Licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---