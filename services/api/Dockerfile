# --- build stage ---
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/server

# --- runtime stage ---
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

COPY --from=builder /api /api

EXPOSE 8080

CMD [ "/api" ]
