# This file is the project's own.
# Add recipes leveraging provided `do` ready-made recipes, or create your own.
# The import must be kept: it mounts every shared limen task under `just do ...`.
import '.limen/just/main.just'

# The FIRST recipe defined here becomes `just`'s default.
lint: do::lint::go::default do::lint::default
fix: do::fix::go::default do::fix::default
test: do::test::go::unit do::test::go::race test-integration
security: do::security::default

# The suite runs the built binary, not the package, so it needs YAMLFMT_BIN.
[doc('Integration tests: the yamlfmt binary against integrationtest/command/testdata')]
test-integration *args:
    #!/usr/bin/env bash
    set -euo pipefail
    bin="$(mktemp -d)"
    trap 'rm -rf "$bin"' EXIT
    exe="$bin/yamlfmt$(go env GOEXE)"
    go build -o "$exe" ./cmd/yamlfmt
    YAMLFMT_BIN="$exe" go test -tags=integration_test ./integrationtest/command {{ args }}
