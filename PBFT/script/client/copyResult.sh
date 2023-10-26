#!/bin/bash

NUM=$1
NAME=$2

for ((i = 0; i < NUM; i++)); do
{
  host1=$(jq ".nodes[$i].PublicIpAddress" clients.json)
  host=${host1//\"/}
  port=5000
  user='ubuntu'
  key="~/.ssh/aws"
  id=$i
  node="node"$id
  scp -i $key $user@$host:client/client$id.output ./log/client$id/$NAME

} &
done

wait
