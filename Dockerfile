# Multi-stage build for minimal production image
# Supports cross-compilation for ARM64 (Raspberry Pi, IoT gateway)

FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /secsgem ./cmd/secsgem/

# --- Production image ---
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /secsgem /usr/local/bin/secsgem

EXPOSE 5000

ENTRYPOINT ["secsgem"]
CMD ["simulate", "--addr", ":5000"]
