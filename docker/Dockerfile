FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git nodejs npm

WORKDIR /app

# Front-end assets
COPY web/package.json ./web/
RUN npm install --prefix ./web && \
    mkdir -p web/static/vendor/shoelace && \
    cp -r web/node_modules/@shoelace-style/shoelace/cdn/. web/static/vendor/shoelace/ && \
    cp web/node_modules/htmx.org/dist/htmx.min.js web/static/vendor/htmx.min.js && \
    rm -rf web/node_modules

# Download Go dependencies (cached layer — only re-runs when go.mod changes)
COPY go.mod ./
RUN go mod download

# Copy source (go.sum excluded by .dockerignore so the downloaded one is preserved)
COPY . .

# Generate templ files
RUN go install github.com/a-h/templ/cmd/templ@v0.3.857 && \
    templ generate

# Tidy ensures go.sum is complete
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o hakken ./cmd/hakken

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 hakken && \
    adduser -u 1000 -G hakken -D -H hakken

WORKDIR /app

COPY --from=builder /app/hakken .
COPY --from=builder /app/web/static ./web/static

RUN chown -R hakken:hakken /app

USER hakken

EXPOSE 8080

CMD ["./hakken"]
