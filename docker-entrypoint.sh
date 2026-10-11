#!/bin/sh
set -e

# Support persistent storage when /app/data volume is mounted
if [ -d /app/data ]; then
    if [ ! -f /app/data/.config.yaml ] && [ -f /app/config.yaml ]; then
        echo "[entrypoint] Initializing /app/data/.config.yaml from default config.yaml..."
        cp /app/config.yaml /app/data/.config.yaml
    fi

    if [ ! -f /app/data/config.db ]; then
        touch /app/data/config.db
    fi

    ln -sf /app/data/.config.yaml /app/.config.yaml
    ln -sf /app/data/config.db /app/config.db
else
    # Standalone mode without /app/data volume
    if [ ! -f /app/.config.yaml ] && [ -f /app/config.yaml ]; then
        echo "[entrypoint] Initializing .config.yaml from default config.yaml..."
        cp /app/config.yaml /app/.config.yaml
    fi
fi

exec "$@"
