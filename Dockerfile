# Stage 1: Build UI
FROM node:20-alpine AS ui-builder

WORKDIR /app
COPY ui/package.json ui/package-lock.json ./ui/
RUN cd ui && npm ci --silent
COPY ui/ ./ui/
COPY internal/ui/ ./internal/ui/
RUN cd ui && npm run build

# Stage 2: Build Go binary with embedded UI
FROM golang:1.24-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
      -X github.com/gshepptech/mozza/internal/version.Version=${VERSION} \
      -X github.com/gshepptech/mozza/internal/version.Commit=${COMMIT} \
      -X github.com/gshepptech/mozza/internal/version.Date=${BUILD_DATE}" \
    -o /bin/mozza ./cmd/mozza

# Stage 3: Minimal runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl
COPY --from=builder /bin/mozza /usr/local/bin/mozza

VOLUME /data
EXPOSE 8080
ENTRYPOINT ["mozza", "serve"]
