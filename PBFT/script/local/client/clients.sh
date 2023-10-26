tmux new -d -s client0 "./client -id 0 -n 4 -reqnum 10 -payload 250 -port 6200 -time 2 -output client0.log > client0.output 2>&1"
tmux new -d -s client1 "./client -id 1 -n 4 -reqnum 10 -payload 250 -port 6201 -time 2 -output client1.log > client1.output 2>&1"
tmux new -d -s client2 "./client -id 2 -n 4 -reqnum 10 -payload 250 -port 6202 -time 2 -output client2.log > client2.output 2>&1"
tmux new -d -s client3 "./client -id 3 -n 4 -reqnum 10 -payload 250 -port 6203 -time 2 -output client3.log > client3.output 2>&1"
