# Multi-stage build for minimal production image
# Supports cross-compilation for ARM64 (Raspberry Pi, IoT gateway)

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /secsgem ./cmd/secsgem/

# --- Production image ---
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /secsgem /usr/local/bin/secsgem

EXPOSE 10000

ENTRYPOINT ["secsgem"]
CMD ["studio", "--port", "10000"]
