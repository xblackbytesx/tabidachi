FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git nodejs npm

WORKDIR /app

# Front-end deps (installed to /tmp so COPY . . can't shadow them)
COPY web/package.json /tmp/web/
RUN npm install --prefix /tmp/web

# Cache Go dependencies (layer re-runs only when go.mod changes)
COPY go.mod ./
RUN go mod download

COPY . .

# Vendor frontend assets after COPY to avoid overlay directory shadowing
RUN mkdir -p web/static/vendor/fontawesome && \
    cp /tmp/web/node_modules/@fortawesome/fontawesome-free/css/all.min.css web/static/vendor/fontawesome/ && \
    cp -r /tmp/web/node_modules/@fortawesome/fontawesome-free/webfonts web/static/vendor/fontawesome/webfonts && \
    cp /tmp/web/node_modules/htmx.org/dist/htmx.min.js web/static/vendor/htmx.min.js && \
    cp /tmp/web/node_modules/sortablejs/Sortable.min.js web/static/vendor/sortable.min.js

# Generate templ files
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1001 && \
    templ generate

# Tidy ensures go.sum is complete
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o tabidachi ./cmd/tabidachi

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 tabidachi && \
    adduser -u 1000 -G tabidachi -D -H tabidachi

WORKDIR /app

COPY --from=builder /app/tabidachi .
COPY --from=builder /app/web/static ./web/static

RUN chown -R tabidachi:tabidachi /app

USER tabidachi

EXPOSE 8080

CMD ["./tabidachi"]
