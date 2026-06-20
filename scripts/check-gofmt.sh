#!/usr/bin/env bash
set -euo pipefail
unformatted="$(go tool gofumpt -l .)"
if [[ -n "${unformatted}" ]]; then
	printf 'gofumpt: the following files are not formatted:\n%s\n' "${unformatted}" >&2
	exit 1
fi
