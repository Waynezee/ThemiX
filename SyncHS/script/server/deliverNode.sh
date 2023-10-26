#!/bin/bash

NUM=$1

for ((i = 0; i < NUM; i++)); do
{
        host1=$(jq ".nodes[$i].PublicIpAddress" nodes.json)
        host=${host1//\"/}
        port=5000
        user='ubuntu'
        key="~/.ssh/aws"
        id=$i
        node="node"$id

        expect <<-END
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;mkdir synchs;mkdir -p synchs/conf;mkdir -p synchs/script;mkdir -p synchs/crypto;mkdir -p synchs/log;cd synchs/log;touch server0"
expect EOF
exit
END

        expect -c "
set timeout -1
spawn scp -i $key ../../src/core/main  $user@$host:synchs/
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key ../../src/crypto/priv_sk $user@$host:synchs/crypto
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key stop.sh $user@$host:synchs/script/
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key $node.json $user@$host:synchs/
expect 100%
exit
"

        expect <<-END
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;chmod 777 synchs/main;cd synchs/script;chmod 777 stop.sh;cd ..;mv $node.json node.json"
expect EOF
exit
END
} &
done

wait
