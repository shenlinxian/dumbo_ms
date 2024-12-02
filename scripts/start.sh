#!/bin/bash
if [ "$1" -eq 1 ]; then
    cd ./experiment/honeybadger_test/ && ./honeybadger_test --id=$2
else
    cd ./experiment/fin/ && ./fin --id=$2
fi