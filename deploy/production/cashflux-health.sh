#!/usr/bin/env sh
set -eu

health_url="${CASHFLUX_HEALTH_URL:-http://127.0.0.1:8105/readyz}"

if curl --fail --silent --show-error --max-time 10 --output /dev/null "$health_url"; then
	exit 0
fi

sleep 30
if curl --fail --silent --show-error --max-time 10 --output /dev/null "$health_url"; then
	exit 0
fi

systemctl reset-failed cashflux.service || true
systemctl restart cashflux.service
