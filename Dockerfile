FROM golang:1.21-alpine AS builder

WORKDIR /build

COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o trish .
RUN CGO_ENABLED=0 GOOS=linux go build -o trish-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o trish-agent ./cmd/agent

FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/trish ./
COPY --from=builder /build/trish-server ./
COPY --from=builder /build/trish-agent ./

ENTRYPOINT ["./trish"]
CMD ["list"]
