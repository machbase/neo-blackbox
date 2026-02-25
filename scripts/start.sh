#!/usr/bin/env bash
set -e

mkdir -p ./.backend

# Run in foreground like neo-cat.
# Store current shell pid first; after exec, the same pid becomes neo-blackbox.
echo $$ > ./.backend/pid
exec ./bin/neo-blackbox -config config/config.yaml > ./.backend/stdout.log 2> ./.backend/stderr.log
