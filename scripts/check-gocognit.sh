#!/usr/bin/env bash
set -euo pipefail
findings="$(go tool gocognit -over 7 .)"
if [[ -n "${findings}" ]]; then
	printf 'gocognit: functions exceed cognitive complexity 7:\n%s\n' "${findings}" >&2
	exit 1
fi
