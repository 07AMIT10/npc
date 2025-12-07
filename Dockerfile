# Build stage for React frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web-react
COPY web-react/package*.json ./
RUN npm ci --only=production
COPY web-react/ ./
RUN npm run build

# Build stage for Go backend
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Production stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy Go binary
COPY --from=backend-builder /app/server .

# Copy React build to web directory (will be served by Go)
COPY --from=frontend-builder /app/web-react/dist ./web

# Copy config
COPY config.yaml .

# Environment
ENV PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./server"]
