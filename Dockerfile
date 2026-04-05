FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/comment-service ./cmd/comment-service

FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/comment-service /app/comment-service
COPY migrations /app/migrations

EXPOSE 8080

ENTRYPOINT ["/app/comment-service"]
