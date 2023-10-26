#!/bin/bash

ps -u `whoami` | grep main | awk '{system("kill -9 "$1)}'
ps -u `whoami` | grep client | awk '{system("kill -9 "$1)}'
