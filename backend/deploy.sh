#!/usr/bin/env bash
# VPS deploy helper for ai-image backend.
#
# Usage (from a workstation):
#   ./deploy.sh user@vps.example.com /opt/ai-image
#
# Prerequisites on the VPS (one-time):
#   - Docker + Docker Compose plugin installed
#   - User in `docker` group
#   - Directory created (e.g. /opt/ai-image) containing:
#       * docker-compose.yml (copy of this repo's backend/docker-compose.yml)
#       * Caddyfile          (copy of this repo's backend/Caddyfile)
#       * .env               (production env vars; see backend/.env.example)
#   - Domain DNS A/AAAA pointing to VPS public IP
#   - Ports 80, 443 open in firewall
#
# What this script does:
#   1. SSH into VPS
#   2. cd into project dir
#   3. docker compose pull   (pulls latest backend image from GHCR)
#   4. docker compose up -d  (recreates containers with new image)
#   5. docker compose ps     (shows status)
#
# Image is built and pushed by GitHub Actions on every push to main.
# To deploy a specific commit, set BACKEND_IMAGE in .env on the VPS:
#   BACKEND_IMAGE=ghcr.io/bimantara/ai-image-backend:abc1234

set -euo pipefail

if [[ $# -lt 2 ]]; then
	echo "Usage: $0 user@host /path/to/project/on/vps"
	exit 1
fi

REMOTE="$1"
REMOTE_DIR="$2"

echo "==> Deploying to $REMOTE:$REMOTE_DIR"

ssh "$REMOTE" "cd $REMOTE_DIR && docker compose pull && docker compose up -d && docker compose ps"

echo "==> Done. Tail logs with:"
echo "   ssh $REMOTE 'cd $REMOTE_DIR && docker compose logs -f backend caddy'"
