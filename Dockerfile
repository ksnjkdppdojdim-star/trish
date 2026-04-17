# Multi-stage build for Trish

# Stage 1: Builder
FROM golang:1.21-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o trish .

# Stage 2: Runtime
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/trish .

ENTRYPOINT ["./trish"]
CMD ["list"]
