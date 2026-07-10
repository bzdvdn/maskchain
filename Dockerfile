FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gateway ./src/cmd/gateway/

FROM gcr.io/distroless/static-debian12

COPY --from=builder /gateway /gateway

ENTRYPOINT ["/gateway"]
