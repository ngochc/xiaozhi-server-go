FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    sqlite-libs \
    procps

WORKDIR /app

CMD ["./xiaozhi-server"]
