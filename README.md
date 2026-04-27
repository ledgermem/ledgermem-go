# ledgermem-go

Official Go SDK for [LedgerMem](https://proofly.dev) — auditable memory for AI agents.

## Install

```bash
go get github.com/ledgermem/ledgermem-go
```

Requires Go 1.22+.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	ledgermem "github.com/ledgermem/ledgermem-go"
)

func main() {
	client := ledgermem.NewClient(ledgermem.Config{
		APIKey:      "lm_live_...",
		WorkspaceID: "ws_123",
	})
	ctx := context.Background()

	mem, err := client.Memories.Add(ctx, ledgermem.AddMemoryInput{
		Content: "User prefers dark mode.",
	})
	if err != nil {
		log.Fatal(err)
	}

	hits, err := client.Search(ctx, ledgermem.SearchInput{Query: "dark mode", Limit: 5})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(mem.ID, len(hits.Hits))
}
```

Configuration falls back to env vars: `LEDGERMEM_API_KEY`, `LEDGERMEM_WORKSPACE_ID`, `LEDGERMEM_API_URL`.

## API

| Method                       | Endpoint                  |
| ---------------------------- | ------------------------- |
| `client.Search`              | `POST /v1/search`         |
| `client.Memories.Add`        | `POST /v1/memories`       |
| `client.Memories.Update`     | `PATCH /v1/memories/:id`  |
| `client.Memories.Delete`     | `DELETE /v1/memories/:id` |
| `client.Memories.List`       | `GET /v1/memories`        |

## License

MIT
