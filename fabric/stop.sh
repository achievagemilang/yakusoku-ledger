#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
export COMPOSE_PROJECT_NAME=yakusoku
docker compose -f docker/docker-compose-sdk.yaml down
