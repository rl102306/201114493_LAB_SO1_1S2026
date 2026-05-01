# ---- Builder ----
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache protobuf protobuf-dev

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0

WORKDIR /app

COPY go.mod ./
COPY . .

# Generate proto Go files
RUN protoc \
    --go_out=. \
    --go-grpc_out=. \
    proto/war_report.proto

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o grpc-client .

# ---- Runtime ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/grpc-client ./grpc-client

EXPOSE 8080
CMD ["./grpc-client"]
