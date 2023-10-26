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
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;mkdir bkr;mkdir -p bkr/conf;mkdir -p bkr/script;mkdir -p bkr/crypto;mkdir -p bkr/log;cd bkr/log;touch server0"
expect EOF
exit
END

        expect -c "
set timeout -1
spawn scp -i $key ../../src/acs/server/cmd/main  $user@$host:bkr/
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key crypto.tar.gz $user@$host:bkr/crypto.tar.gz
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key stop.sh $user@$host:bkr/script/
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key $node.json $user@$host:bkr/
expect 100%
exit
"

        expect <<-END
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;chmod 777 bkr/main;cd bkr/script;chmod 777 stop.sh;cd ..;mv $node.json node.json;rm -rf crypto;tar -xvf crypto.tar.gz"
expect EOF
exit
END
} &
done

wait
