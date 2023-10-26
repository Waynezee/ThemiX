#!/bin/bash

NUM=$1

for ((i = 0; i < NUM; i++)); do

{
  host1=$(jq ".nodes[$i].PublicIpAddress" nodes.json)
  host=${host1//\"/}
  port=6000
  user='ubuntu'
  key="~/.ssh/aws"
  id=$i
  node="node"$id
  cmd="cd;cd synchs;nohup ./main --batch $2 > /dev/null 2>&1 &"

expect <<-END
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;cd synchs;nohup ./main -conf ./node.json -debug=true > server$i.output 2>&1 &"
expect EOF
exit
END
} &
done

wait
