#!/usr/bin/env bash
set +e

if [ -e ./.backend/pid ]
then
    pid=$(cat ./.backend/pid)
    if [ -n "${pid}" ]
    then
        kill "${pid}" 2>/dev/null || true
    fi
fi

# Fallback for stale pid files: terminate listeners owned by neo-blackbox.
for p in $(lsof -tiTCP:8000 -sTCP:LISTEN 2>/dev/null); do
    cmd=$(ps -p "${p}" -o comm= 2>/dev/null || true)
    if [ "${cmd}" = "neo-black" ] || [ "${cmd}" = "neo-blackbox" ]; then
        kill "${p}" 2>/dev/null || true
    fi
done
