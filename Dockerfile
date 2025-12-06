# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --only=production=false
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
COPY go.mod ./
RUN go mod tidy
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o oci-panel main.go

# Stage 3: Final minimal image
FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=backend-builder /app/oci-panel .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
COPY config.toml.example ./config.toml
EXPOSE 8818
CMD ["./oci-panel"]
