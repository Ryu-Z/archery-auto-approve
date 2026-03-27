FROM golang:1.21-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/archery-auto-approve ./main.go

FROM debian:stable-slim

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system app \
    && useradd --system --gid app --create-home --home-dir /home/app --shell /usr/sbin/nologin app

ENV TZ=Asia/Shanghai

COPY --from=builder /out/archery-auto-approve /app/archery-auto-approve
COPY config.yaml /app/config.yaml
RUN chown -R app:app /app

USER app

EXPOSE 8080

ENTRYPOINT ["/app/archery-auto-approve"]
