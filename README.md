# cbar-rates

A small Go microservice that fetches the official daily exchange rates from the
Central Bank of Azerbaijan (CBAR), caches them, and serves them as a JSON API.

> Part of a 3-service system: [az-job-radar](https://github.com/saidmuradkhan/az-job-radar) (Python) · **cbar-rates** (Go) · [jobtrack](https://github.com/saidmuradkhan/jobtrack) (Django + React)

## API

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness check |
| GET | `/rates` | All rates for today |
| GET | `/rates/{code}` | One currency, e.g. `/rates/USD` |
| GET | `/convert?from=USD&to=AZN&amount=1500` | Convert an amount |

All rates are in AZN. Some currencies (JPY, RUB, ...) are quoted by CBAR per 100 units,
so each rate also has a `per_unit` field.

```
$ curl "localhost:8080/convert?from=USD&to=AZN&amount=1500"
{"amount":1500,"date":"2026-10-05","from":"USD","rate":1.7,"result":2550,"to":"AZN"}
```

## Run locally

```
go run .                # http://localhost:8080 (set PORT to change)
go test ./...
```

## Tech stack

Go (standard library `net/http`, `encoding/xml`) · Docker · GitHub Actions

## Roadmap

- [x] `go mod init`, hello-world HTTP server with `/health`
- [x] Fetch CBAR daily XML and parse it into Go structs
- [x] `/rates` and `/rates/{code}` endpoints
- [x] In-memory cache (1 hour TTL, serves stale data if CBAR is down)
- [x] `/convert` endpoint
- [x] Unit tests with `testing` + `httptest`
- [ ] Graceful shutdown, structured logging (`log/slog`)
- [ ] Dockerfile (multi-stage build)
- [ ] CI: `go vet`, `go test`

## License

MIT
