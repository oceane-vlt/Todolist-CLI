# Multi-stage build for the TodoList gRPC server (Phase 6 — remote hosting).
# Only the server binary is deployed; the CLI runs on the user's machine.

# ---- build stage ----
FROM golang:1.24-alpine AS build

WORKDIR /src

# Cache module downloads separately from the source for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static build so the binary runs on a minimal base image.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---- runtime stage ----
FROM alpine:3.20

# ca-certificates so the server can validate TLS endpoints (e.g. Supabase JWKS
# in a future hardening step) and reach a managed Postgres over TLS.
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app

COPY --from=build /out/server /usr/local/bin/server

USER app

# Fly.io injects PORT; the server binds 0.0.0.0:$PORT (see server/listenaddr.go).
# Documented default for local docker runs; the platform overrides it.
ENV PORT=50051
EXPOSE 50051

ENTRYPOINT ["/usr/local/bin/server"]
