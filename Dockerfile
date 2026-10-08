FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /cbar-rates .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /cbar-rates /cbar-rates
ENV PORT=8080
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/cbar-rates"]
