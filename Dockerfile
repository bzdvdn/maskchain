# @sk-task 41-profiles-ui#T1.3: Multi-stage Dockerfile with UI build (AC-009)
# Stage 1: UI build
FROM node:20-alpine AS ui-builder

WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ .
RUN npm run build

# Stage 2: Go build
FROM golang:1.26-alpine AS go-builder

WORKDIR /app
COPY . .
COPY --from=ui-builder /app/ui/dist ./ui/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gateway ./src/cmd/gateway/

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12

COPY --from=go-builder /gateway /gateway

ENTRYPOINT ["/gateway"]
