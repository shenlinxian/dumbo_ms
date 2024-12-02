#!/bin/bash

PID=$(pidof "$1")

while true; do
    MEM_USAGE=$(ps -p $PID -o rss=)

    if [ -z "$MEM_USAGE" ]; then
        echo "Process with PID $PID not found."
        break
    fi

    CURRENT_TIME=$(date "+%Y-%m-%d %H:%M:%S")

    echo "$CURRENT_TIME - PID: $PID - Memory Usage: $MEM_USAGE bytes" >> memorylog.txt

    sleep 1
done
