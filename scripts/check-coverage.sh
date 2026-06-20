#!/usr/bin/env bash
set -euo pipefail
profile="$(mktemp)"
trap 'rm -f "${profile}"' EXIT
go test -covermode=set -coverprofile="${profile}" ./...
total="$(go tool cover -func="${profile}" | awk '/^total:/ {print $3}')"
if [[ "${total}" != "100.0%" ]]; then
	printf 'coverage: total is %s, want 100.0%%\n' "${total}" >&2
	exit 1
fi
printf 'coverage: %s\n' "${total}"
