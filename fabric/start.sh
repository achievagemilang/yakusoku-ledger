#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -d channel-artifacts/certs ] || ! find channel-artifacts/certs -name '*_sk' -print -quit | grep -q .; then
	bash ./generate-artifacts.sh
fi

export COMPOSE_PROJECT_NAME=yakusoku
docker compose -f docker/docker-compose-sdk.yaml up -d
