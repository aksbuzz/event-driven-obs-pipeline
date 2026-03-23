FROM golang:1.26 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
  gcc libc6-dev librdkafka-dev pkg-config

WORKDIR /app
COPY services/enricher/go.mod services/enricher/go.su[m] ./
RUN go mod download

COPY services/enricher/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -o enricher ./main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
  librdkafka1 ca-certificates && \
  rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/enricher .
COPY libs/event-schema/service-registry.json ./service-registry.json
COPY infra/geoip/GeoLite2-City.mmdb ./GeoLite2-City.mmdb

EXPOSE 9090
ENTRYPOINT ["./enricher"]