FROM golang:1.24.1-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy-scraper-checker .
FROM alpine:3.16
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/proxy-scraper-checker /app/
COPY config.yaml /app/
COPY sources/ /app/sources/
RUN mkdir -p /app/out
ENTRYPOINT ["/app/proxy-scraper-checker"]