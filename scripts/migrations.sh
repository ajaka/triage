#!/usr/bin/env bash
set -e

if [ -f .env ]; then
  set -a
  . .env
  set +a
fi

case "${1:-}" in
  up)
    migrate -path migrations -database "$DATABASE_URL" up
    ;;
  down)
    count="${2:-1}"
    echo "Rolling back ${count} migration(s). Continue? [y/N]"
    read -r confirm
    if [ "$confirm" = "y" ]; then
      migrate -path migrations -database "$DATABASE_URL" down "$count"
    fi
    ;;
  create)
    migrate create -ext sql -dir migrations -seq "${2:-}"
    ;;
  force)
    migrate -path migrations -database "$DATABASE_URL" force "${2:-}"
    ;;
  *)
    echo "Unknown command"
    ;;
esac
