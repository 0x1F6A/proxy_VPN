#!/usr/bin/env bash
# failover.sh — promote a MySQL read replica to primary when the primary
# region is unavailable. This is a *manual* runbook helper: review every
# step before running in production. We intentionally do not automate
# DNS/health-based failover because the false-positive cost is huge for
# a payment-processing system.
#
# Usage:
#   ./failover.sh check  <replica-host>
#   ./failover.sh promote <replica-host> <root-password>
#
# Prereq: the replica was started with --read-only=ON and is healthy.
set -euo pipefail

cmd="${1:-}"
host="${2:-}"
pass="${3:-}"

usage() {
  cat <<EOF
Usage:
  $0 check   <replica-host>
  $0 promote <replica-host> <root-password>
EOF
  exit 1
}

[[ -z "$cmd" || -z "$host" ]] && usage

case "$cmd" in
  check)
    echo "==> replication lag on $host"
    mysql -h "$host" -uroot -p"${MYSQL_ROOT_PASSWORD:-}" -e "SHOW REPLICA STATUS\G" \
      | grep -E "Seconds_Behind_Source|Replica_IO_Running|Replica_SQL_Running|Last_Error" || true
    ;;
  promote)
    [[ -z "$pass" ]] && usage
    echo "==> stopping replication on $host"
    mysql -h "$host" -uroot -p"$pass" -e "STOP REPLICA;"
    echo "==> resetting replica metadata"
    mysql -h "$host" -uroot -p"$pass" -e "RESET REPLICA ALL;"
    echo "==> turning OFF read_only"
    mysql -h "$host" -uroot -p"$pass" -e "SET GLOBAL read_only=OFF; SET GLOBAL super_read_only=OFF;"
    echo "==> verify"
    mysql -h "$host" -uroot -p"$pass" -e "SELECT @@read_only, @@super_read_only, @@hostname;"
    cat <<EOF

PROMOTED. Next steps (do these in order, do NOT skip):
  1. Update the application DSN to point PROXYVPN_DB__DSN at $host
  2. Update PROXYVPN_DB__READ_REPLICAS to remove $host (it is primary now)
  3. Restart pods (kubectl rollout restart deploy)
  4. Update DNS / GLB to send all traffic to this region
  5. Capture binlog position on the new primary for re-seeding the old
     primary as a fresh replica once it is recovered.
EOF
    ;;
  *)
    usage
    ;;
esac
