#!/bin/bash

PID=$(pidof "$1")

while true; do
    MEM_USAGE=$(ps -p $PID -o rss=)

    if [ -z "$MEM_USAGE" ]; then
        echo "Process with PID $PID not found."
        break
    fi

    CURRENT_TIME=$(date +'%T')

    echo "$CURRENT_TIME - Memory Usage: $MEM_USAGE" >> memorylog.txt

    sleep 1
done
