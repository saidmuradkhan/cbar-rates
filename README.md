# cbar-rates

![CI](https://github.com/saidmuradkhan/cbar-rates/actions/workflows/ci.yml/badge.svg)

A small Go microservice that fetches the official daily exchange rates from the
Central Bank of Azerbaijan (CBAR), caches them, and serves them as a JSON API
plus a small converter page.

**Live demo:** [rates.saidmuradkhan.dev](https://rates.saidmuradkhan.dev) · [USD rate](https://rates.saidmuradkhan.dev/rates/USD) *(private preview, login required for now)*

> Part of a 3-service system: [az-job-radar](https://github.com/saidmuradkhan/az-job-radar) (Python) · **cbar-rates** (Go) · [jobtrack](https://github.com/saidmuradkhan/jobtrack) (Django + React)

## API

| Method | Path | Description |
|---|---|---|
| GET | `/` | Converter page with a 30-day chart (HTML, works on mobile) |
| GET | `/api` | List of endpoints (JSON) |
| GET | `/health` | Liveness check |
| GET | `/rates` | All rates for today |
| GET | `/rates/{code}` | One currency, e.g. `/rates/USD` |
| GET | `/convert?from=USD&to=AZN&amount=1500` | Convert an amount |
| GET | `/history/{code}?days=30` | Rate of one currency for the last 1–90 days |

All rates are in AZN. Some currencies (JPY, RUB, ...) are quoted by CBAR per 100 units,
so each rate also has a `per_unit` field.

`/rates`, `/rates/{code}` and `/convert` also take `?date=YYYY-MM-DD` for a past day.
CBAR does not publish rates on weekends and holidays, so those days return the last
working day's rates (the `date` field shows which one).

### Examples

```
$ curl "localhost:8080/convert?from=USD&to=AZN&amount=1500"
{"amount":1500,"date":"2026-10-08","from":"USD","rate":1.7,"result":2550,"to":"AZN"}

$ curl "localhost:8080/rates/EUR?date=2026-09-15"
{"base":"AZN","date":"2026-09-15","rate":{"code":"EUR","name":"1 Avro","nominal":1,"value":1.9613,"per_unit":1.9613}}

$ curl "localhost:8080/history/EUR?days=3"
{"base":"AZN","code":"EUR","points":[{"date":"2026-10-06","per_unit":1.9069},{"date":"2026-10-07","per_unit":1.9095},{"date":"2026-10-08","per_unit":1.9048}]}

$ curl "localhost:8080/convert?from=USD&to=AZN&amount=abc"
{"error":"amount must be a non-negative number"}
```

## Run locally

```
go run .                # http://localhost:8080 (set PORT to change)
go test ./...
```

Or with Docker (multi-stage build, the final image is a ~20 MB distroless image
that runs as a non-root user):

```
docker build -t cbar-rates .
docker run -p 8080:8080 cbar-rates
```

Logs are JSON lines (`log/slog`), one per request:

```
{"time":"...","level":"INFO","msg":"request","method":"GET","path":"/rates/USD","status":200,"duration_ms":0}
```

## Deployment

Runs on Vercel with the Go framework preset (`vercel.json`), straight from `main.go`.
The Dockerfile is there for any other host (Fly.io, Render, a VPS).

While the project is in review, the site is behind a small login page, controlled by
environment variables:

| Variable | Purpose |
|---|---|
| `PREVIEW_USER`, `PREVIEW_PASSWORD` | Login credentials. If either is missing, the site is public. |
| `PREVIEW_SECRET` | Key for signing the session cookie. Other services can send it as `X-Preview-Token`. |

`/health` is always public.

## Tech stack

Go (standard library `net/http`, `encoding/xml`, `log/slog`, `embed`) · plain SVG chart, no JS libraries · Docker · GitHub Actions · Vercel

## Roadmap

- [x] `go mod init`, hello-world HTTP server with `/health`
- [x] Fetch CBAR daily XML and parse it into Go structs
- [x] `/rates` and `/rates/{code}` endpoints
- [x] In-memory cache (1 hour TTL, serves stale data if CBAR is down)
- [x] `/convert` endpoint
- [x] Unit tests with `testing` + `httptest`
- [x] Preview deployment on Vercel behind a login page
- [x] Converter page at `/`
- [x] Structured logging (`log/slog`), one line per request
- [x] CI: `gofmt`, `go vet`, `go test -race`
- [x] Historical rates (`/rates?date=`) and a 30-day chart
- [ ] Graceful shutdown, rate limiting
- [x] Dockerfile (multi-stage build)

## License

MIT
