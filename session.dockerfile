FROM docker.io/library/golang:1.22 AS build-env
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    musl-tools \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY . .
RUN go mod tidy && CGO_ENABLED=1 GOOS=linux \
    CC=musl-gcc go build -ldflags="-w -s" -o /app/bin/rvc-session ./cmd/session/main.go

FROM docker.io/library/alpine:3.19
RUN apk --no-cache add ca-certificates
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=build-env /app/bin/rvc-session /app/bin/rvc-session
ENTRYPOINT ["/app/bin/rvc-session"]