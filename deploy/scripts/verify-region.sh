#!/usr/bin/env bash
# verify-region.sh — probe the public API endpoints of a region and report
# whether the control plane is healthy enough to accept traffic. Intended
# to be wired into Cloudflare / Route53 / external uptime monitors as a
# *single* unified health URL aggregator.
#
# Usage:
#   ./verify-region.sh https://api.us-east.example.com
set -euo pipefail

base="${1:-}"
if [[ -z "$base" ]]; then
  echo "usage: $0 <api-base-url>"
  exit 1
fi

fail=0

probe() {
  local path="$1"
  local code
  code=$(curl -s -o /tmp/.verify.$$ -w "%{http_code}" -m 5 "$base$path" || echo "000")
  if [[ "$code" == "200" ]]; then
    echo "[OK]   $path -> $code"
  else
    echo "[FAIL] $path -> $code"
    head -c 300 /tmp/.verify.$$ || true
    echo
    fail=1
  fi
  rm -f /tmp/.verify.$$
}

probe /healthz
probe /readyz

# readyz body should include mysql.write=ok and (if replicas configured)
# mysql.read=ok. Print the breakdown for operators.
echo "---- /readyz body ----"
curl -s -m 5 "$base/readyz" | head -c 500
echo

exit "$fail"
