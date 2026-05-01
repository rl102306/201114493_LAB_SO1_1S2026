# ---- Builder ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o consumer .

# ---- Runtime ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/consumer ./consumer

CMD ["./consumer"]
