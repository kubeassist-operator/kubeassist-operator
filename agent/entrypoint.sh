#!/usr/bin/env sh
set -e
exec uvicorn server:app \
  --host 0.0.0.0 \
  --port 8080 \
  --workers 1 \
  --log-level info
