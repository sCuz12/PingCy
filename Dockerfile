# ---------- Frontend build (Vite) ----------
FROM node:20-alpine AS web-builder
WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build


# ---------- Backend build (Go) ----------
FROM golang:1.25-alpine AS go-builder
WORKDIR /app/backend

RUN apk add --no-cache ca-certificates git

# Cache deps (go.mod + go.sum live under /backend)
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source
COPY backend/ ./

# Copy built frontend into backend so Go can serve it (e.g. ./web/dist)
COPY --from=web-builder /app/web/dist ./web/dist

# Build binary (adjust if your main package path differs)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /bin/cyping ./server


# ---------- Runtime ----------
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates && update-ca-certificates

RUN addgroup -S app && adduser -S app -G app

COPY --from=go-builder /bin/cyping /app/cyping
COPY --from=go-builder /app/backend/configs /app/configs
COPY --from=go-builder /app/backend/web/dist /app/web/dist

USER app
EXPOSE 8080

ENV CONFIG_PATH=/app/configs/config.yaml

CMD ["/app/cyping"]