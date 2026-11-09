#!/usr/bin/env bash
# Enforce 100% statement coverage, reached honestly.
set -euo pipefail

profile=$(mktemp)
filtered=$(mktemp)
trap 'rm -f "$profile" "$filtered"' EXIT

go test ./evals -coverprofile="$profile" > /dev/null
awk -v prefix="$(go list -m)/" '
    NR == 1 { print; next }
    {
        split($1, location, ":")
        file = substr(location[1], length(prefix) + 1)
        split(location[2], position, ".")
        if (!(file in loaded)) {
            line = 0
            while ((getline text < file) > 0) source[file, ++line] = text
            close(file)
            loaded[file] = 1
        }
        if (source[file, position[1]] ~ /\/\/ pragma: no cover - [[:alnum:]]/) next
        print
    }
' "$profile" > "$filtered"
total=$(go tool cover -func="$filtered" | tail -1 | awk '{print $3}')
if [ "$total" != "100.0%" ]; then
    echo "coverage for ./evals is $total, expected 100.0%"
    go tool cover -func="$filtered" | awk '$3+0 < 100 && $1 != "total:"'
    exit 1
fi
