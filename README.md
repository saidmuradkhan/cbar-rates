# cbar-rates

A small Go microservice that fetches the official daily exchange rates from the
Central Bank of Azerbaijan (CBAR), caches them, and serves them as a JSON API.

> Part of a 3-service system: [az-job-radar](https://github.com/saidmuradkhan/az-job-radar) (Python) · **cbar-rates** (Go) · [jobtrack](https://github.com/saidmuradkhan/jobtrack) (Django + React)

## Planned API

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness check |
| GET | `/rates` | All rates for today |
| GET | `/rates/{code}` | One currency, e.g. `/rates/USD` |
| GET | `/convert?from=USD&to=AZN&amount=1500` | Convert an amount |

## Tech stack

Go (standard library `net/http`, `encoding/xml`) · Docker · GitHub Actions

## Roadmap

- [ ] `go mod init`, hello-world HTTP server with `/health`
- [ ] Fetch CBAR daily XML and parse it into Go structs
- [ ] `/rates` and `/rates/{code}` endpoints
- [ ] In-memory cache with a refresh goroutine
- [ ] `/convert` endpoint
- [ ] Unit tests with `testing` + `httptest`
- [ ] Graceful shutdown, structured logging (`log/slog`)
- [ ] Dockerfile (multi-stage build)
- [ ] CI: `go vet`, `go test`

## License

MIT
