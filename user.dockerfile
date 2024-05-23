FROM docker.io/library/golang:1.22 AS build-env
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    musl-tools \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY . .
RUN go mod tidy && CGO_ENABLED=1 GOOS=linux \
    CC=musl-gcc go build -ldflags="-w -s" -o /app/bin/rvc-user ./cmd/user/main.go

FROM docker.io/library/alpine:3.19
RUN apk --no-cache add ca-certificates
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=build-env /app/bin/rvc-user /app/bin/rvc-user
COPY --from=build-env /app/web/ /app/web/
ENTRYPOINT ["/app/bin/rvc-user"]