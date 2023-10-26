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
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;mkdir themix;mkdir -p themix/conf;mkdir -p themix/script;mkdir -p themix/crypto;mkdir -p themix/log;cd themix/log;touch server0"
expect EOF
exit
END

        expect -c "
set timeout -1
spawn scp -i $key ../../src/themix/main  $user@$host:themix/
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key crypto.tar.gz $user@$host:themix/crypto.tar.gz
expect 100%
exit
"

        expect -c "
set timeout -1
spawn scp -i $key stop.sh $user@$host:themix/script/
expect 100%
exit
"
        expect -c "
set timeout -1
spawn scp -i $key $node.json $user@$host:themix/
expect 100%
exit
"
        expect <<-END
spawn ssh -oStrictHostKeyChecking=no -i $key $user@$host "cd;chmod 777 themix/main;cd themix/script;chmod 777 stop.sh;cd ..;mv $node.json node.json;rm -rf crypto;tar -xvf crypto.tar.gz"
expect EOF
exit
END
} &
done

wait
