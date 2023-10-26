tmux new -d -s node0 "./main -conf ./node0.json -debug true -batch 100 > server0.output 2>&1"
tmux new -d -s node1 "./main -conf ./node1.json -debug true -batch 100 > server1.output 2>&1"
tmux new -d -s node2 "./main -conf ./node2.json -debug true -batch 100 > server2.output 2>&1"
tmux new -d -s node3 "./main -conf ./node3.json -debug true -batch 100 > server3.output 2>&1"