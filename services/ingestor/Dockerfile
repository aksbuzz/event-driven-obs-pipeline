FROM golang:1.26 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
  gcc libc6-dev librdkafka-dev pkg-config

WORKDIR /app
COPY services/ingestor/go.mod services/ingestor/go.su[m] ./
RUN go mod download

COPY services/ingestor/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -o ingestor ./main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
  librdkafka1 ca-certificates && \
  rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/ingestor .
COPY libs/event-schema/schema.jso[n] ./schema.json

EXPOSE 8080
ENTRYPOINT ["./ingestor"]