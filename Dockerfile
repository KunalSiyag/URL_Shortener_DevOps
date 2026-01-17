FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o url-shortener ./cmd/server


FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/url-shortener .

EXPOSE 8080

ENV PORT=8080

CMD ["./url-shortener"]

