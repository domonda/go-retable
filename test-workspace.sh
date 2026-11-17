#!/bin/bash
# Runs the tests of every module of the Go workspace (go.work).
# pipefail makes a failing "go list" fail the script
# instead of silently testing nothing.
set -euo pipefail

go list -f '{{.Dir}}' -m | xargs -I {} go test {}/...
