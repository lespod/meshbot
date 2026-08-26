#!/bin/sh
set -eu

mkdir -p /app/config
if [ ! -f /app/config/config.json ]; then
    echo "Copying default config to /app/config/config.json"
    cp /app/default-config/config.json /app/config/config.json
fi

ln -sfn /app/config/config.json /app/config.json
exec /app/meshbot
