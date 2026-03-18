# Instagram MCP Server (Go)

A production-ready MCP server in Go that connects AI assistants to your Instagram Business/Creator account via the Instagram Graph API.

## Project Structure

```
instagram-mcp/
├── cmd/
│   └── instagram-mcp/
│       └── main.go                  # Entrypoint: config → graphapi → instagram → tools → run
│
├── internal/
│   ├── config/
│   │   ├── config.go                # Env loading + validation
│   │   └── config_test.go           # Unit tests
│   │
│   ├── graphapi/                    # ── Reusable HTTP layer ──
│   │   ├── client.go                # Generic Graph API client (GET/POST, auth, error parsing)
│   │   └── errors.go                # Typed APIError with IsRateLimit(), IsAuthError()
│   │
│   ├── instagram/                   # ── Domain layer ──
│   │   ├── types.go                 # Domain types: ReelParams, ImageParams, Insight, etc.
│   │   ├── client.go                # Client interface + NewClient constructor
│   │   ├── container.go             # CreateReelContainer, CreateImageContainer, CreateCarouselContainer
│   │   ├── publish.go               # GetContainerStatus, WaitForContainer, Publish
│   │   ├── insights.go              # GetMediaInsights, GetPublishingRateLimit, ListRecentMedia
│   │   └── mock.go                  # MockClient for unit testing
│   │
│   └── tools/                       # ── MCP tool adapters ──
│       ├── register.go              # Register() — wires all tools to the server
│       ├── helpers.go               # Shared errorResult(), toJSON()
│       ├── upload.go                # upload_reel, upload_image_post, upload_carousel
│       ├── lifecycle.go             # check_container_status, publish_media
│       └── query.go                 # get_media_insights, get_publishing_rate_limit, list_recent_media
│
├── .env.example
├── .gitignore
├── .golangci.yml                    # Linter configuration
├── Dockerfile                       # Multi-stage scratch build
├── go.mod
├── Makefile
└── README.md
```

## Design Principles

**Layered architecture** — Four packages with one-way dependencies:

```
main → tools → instagram → graphapi
                    ↓
                  config
```

- `graphapi` knows nothing about Instagram. It's a reusable Facebook Graph API HTTP client.
- `instagram` adds domain logic (Reel containers, polling, publishing) on top of `graphapi`.
- `tools` maps MCP tool schemas to `instagram.Client` calls. No HTTP knowledge.
- `main` wires everything together and runs the stdio transport.

**Interface-driven testability** — `instagram.Client` is an interface. Every tool can be tested using `instagram.MockClient` without any HTTP calls. The `graphapi.Client` accepts `WithHTTPClient()` for transport-level test injection.

**Single Responsibility per file** — Each file in `instagram/` owns one concern: types, container creation, publish lifecycle, or query operations.

**Typed errors** — `graphapi.APIError` has methods like `IsRateLimit()` and `IsAuthError()` so callers can handle specific failure modes.

**Generic form helpers** — `setOptionalString()` and `setOptionalJSON[T]()` eliminate repetitive nil/empty checks in container creation.

## Tools

| Tool | File | Description |
|------|------|-------------|
| `upload_reel` | upload.go | Full lifecycle: container → poll → publish |
| `upload_image_post` | upload.go | Single JPEG image post |
| `upload_carousel` | upload.go | Combine 2–10 containers into a carousel |
| `check_container_status` | lifecycle.go | Poll container processing status |
| `publish_media` | lifecycle.go | Manually publish a FINISHED container |
| `get_publishing_rate_limit` | query.go | Check 24h publishing quota |
| `get_media_insights` | query.go | Engagement metrics for a published post |
| `list_recent_media` | query.go | List recent account media |

## Quick Start

```bash
# Build
make build

# Configure
cp .env.example .env
# Edit .env with your credentials

# Run
source .env && make run
```

## MCP Host Configuration

### Claude Desktop

```json
{
  "mcpServers": {
    "instagram": {
      "command": "/path/to/bin/instagram-mcp",
      "env": {
        "INSTAGRAM_ACCESS_TOKEN": "EAAxxxxxxx...",
        "INSTAGRAM_ACCOUNT_ID": "17841400123456789"
      }
    }
  }
}
```

### Claude Code

```bash
claude mcp add instagram /path/to/bin/instagram-mcp
```

### Docker

```bash
make docker
docker run --rm \
  -e INSTAGRAM_ACCESS_TOKEN=xxx \
  -e INSTAGRAM_ACCOUNT_ID=yyy \
  instagram-mcp:latest
```

## Development

```bash
make test       # Tests with race detector + coverage
make lint       # golangci-lint
make fmt        # gofmt + goimports
make fmt-check  # CI-friendly format check
```

## Prerequisites

1. Instagram Business/Creator Account linked to a Facebook Page
2. Facebook Developer App with Facebook Login + Instagram Graph API
3. Access token with: `instagram_basic`, `instagram_content_publish`, `pages_read_engagement`
4. Go 1.23+

## Rate Limits

- 50 API-published posts per 24-hour rolling window
- Video processing: 30s–5 min depending on file size
- Use `get_publishing_rate_limit` to monitor quota

## License

MIT
