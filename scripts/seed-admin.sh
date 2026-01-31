#!/usr/bin/env bash
# Create or update the admin console user: username=cashflowAdmin, password=C@$$flowAdmin
# Admin users have role_id=0 and role='A'; backend returns role "Admin" for login.
#
# Usage (from backend directory):
#   Set DB_* env vars (or use .env): DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME_2
#   ./scripts/seed-admin.sh
#
# Then log in to the admin app (admin-cashflow.web.app) with:
#   username: cashflowAdmin
#   password: C@$$flowAdmin

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$BACKEND_DIR"

go run ./cmd/seed-admin
