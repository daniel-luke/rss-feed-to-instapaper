FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /syncer ./cmd/syncer

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

COPY --from=builder /syncer /syncer

ENTRYPOINT ["/syncer"]
