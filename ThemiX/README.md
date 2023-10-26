# ThemiX

## Introduction
This is an implementation of `ThemiX: a novel timing-balanced consensus protocol`.

## Content
```
.
|- Makefile
|- README.md        # Introduction for the implementation of the protocol
|- src
|   |- client       # client: send request to node
|   |- crypto       # crypto primitives: Hash, Threshold Signature, Signature
|   |- Makefile
|   |- themix       # core of protocol
|   |- transport    # network layer: p2p network among servers, communication between client and server
|
|- script
    |- client       # scripts of deliver and start clients, download and analysis logs 
    |- server       # scripts of deliver, start and stop nodes, download and clear logs
```

## Getting Started
### Environment Setup
* Golang 1.19

* install `expect`, `jq`
  ```
  sudo apt update
  sudo apt install -y expect jq
  ```

* python3+
  ```bash
  # boto3 offers AWS APIs which we can use to access the service of AWS in a shell 
  pip3 install boto3==1.16.0

  pip3 install numpy
  ```

* protobuf, the implementation uses protobuf to serialize messages, refer [this](https://github.com/protocolbuffers/protobuf) to get a pre-built binary. (libprotoc 3.14.0 will be ok) 

* install protoc-gen-go and  it must be in your $PATH for the protocol buffer compiler to find it.
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  ```
  
* We also provide a ready protobuf file compiled in Ubuntu 22.04 x86 platform

### Build ThemiX
You can build the whole project as follows
```bash
cd src/
make
```
We set `n=3 and t=2` as default. You can refer to [Build Step by Step](#build-step-by-step) for more information.

### Build Step by Step
1. Build ThemiX
    ```bash
    # Download dependencies.
    cd src/themix
    go get -v -t -d ./...

    # Build ThemiX
    go build -o main server/cmd/main.go
    ```
    - Verification of client request signature is set to `false` as default. You can use flag `-sign true` to change it.
    - $2\Delta$ is set as 2500ms and $2\delta$ is set to 500ms as default. You can refer to [`server/instance.go`](src/themix/server/instance.go) line 99 and 100 to change them.
2. Generate BLS Keys and ECDSA Key
    ```bash
    # Download dependencies.
    cd src/crypto
    go get -v -t -d ./...

    # Generate bls keys.
    # Default n=3, t=2
    go run cmd/bls/main.go -n 3 -t 2

    # Generate ECDSA key.
    go run cmd/ecdsa/main.go
    ```
3. Build Client
    ```bash
    # Compile protobuf.
    cd src/client/proto
    make

    # Build Client.
    cd src/client
    go build -o client main.go
    ```

## Testing

**First of all, you are supposed to change `key` in every shell script to the path of your own ssh key to access your AWS account.** Our access key is named `aws`, and you can change it to your own key.

### Run ThemiX on a cluster of AWS EC2 machines
1. **Create a cluster of AWS EC2 machines.**  
For example, in our three nodes test, we create three `t3.2xlarge` instances in region `us-east-1`, `us-west-1`, `ap-northeast-1` and open `Port 5000-7000`.

2. **Fetch machine information from AWS.** First, `script/server/aws.py` is supposed to be modified:
      - Variable `regions` should contain the regions of your instances
      - Variable `Filter` should be changed in your setting.
    
    After modification, you can fetch instance information as following
    ```bash
    cd script/server
    python3 aws.py
    ```
    

3. **Generate config file(`node.json`) for every node.**
    ```bash
    python3 generate.py
    ```

4. (if not use AWS) add your machines' ipAddr to the variable `ipset` in generateLocal.py, and then
    ```bash
    python3 generateLocal.py
    ```

4. **Deliver nodes.** Again, value `key` in every shell script is supposed to be the path of your AWS secret key to access other instances.
    ```bash
    chmod +x *.sh

    # Compress BLS keys.
    ./tarKeys.sh
    
    # Deliver to every node.
    # n is the number of nodes in the cluster.
    ./deliverNode.sh n
    ```

5. **Run nodes.**
   ```bash
   # <batchsize> makes sense when flag "-sign true" is set. If "-sign" is false, the batchsize can be set arbitrarily.
   # example: ./beginNode.sh n 1
   ./beginNode.sh n <batchsize>
   ```

6. **Stop nodes.**
   ```bash
   ./stopNode.sh n
   ```

### Run clients to send requests to ThemiX nodes.
1. Deliver client.
   ```bash
   chmod +x *.sh
   
   # n is the number of nodes in the cluster
   ./deliverClient.sh n
   ```

2. Run client for a period of time.
   ```bash
   # example: ./beginClient.sh 4 600 10 30
   # you can increase <size of batch> to add the load
   ./beginClient.sh <size of payload> <size of batch> <running time>
   ```

3. Copy result from client node.
   ```bash
   ./createDir.sh n
   ./copyResult.sh n <name of log file>
   ```

4. Calculate throughput and latency.
   ```bash
   # Total bytes sent by client once equal <size of payload> * <size of batch>
   # example: python3 cal.py 4 100 [working_directory]/srcipt/client/log test 30
   python3 cal.py <number of nodes> <batchsize> <path> <name of log file>
   ```

### A Brief introduction of test scripts

#### script/server

* aws.py: get machine information from AWS.
  > You may need to change `regions`and `Filter`.

* generate.py：generate configuration for every node.

* tarKeys.sh: compress BLS keys.

* deliverNode.sh: deliver node to remote machines.
  * `./deliverNode.sh <the number of remote machines>`

* beginNode.sh: run node on remote machines.
  * `./beginNode.sh <the number of remote machines>`

* stopNode.sh: stop node on remote machines.
  * `./stopNode.sh <the number of remote machines>`

* Simulate the crash of specific nodes.
  * You can refer to `crash33.sh`, and change `ADDR` to your specific situation.
  * `./crash33.sh`

* Simulate the crash of the last few nodes.
  * example: simulate the crash of the last 33 nodes of 100 nodes.
    ```bash
    ./crash.sh 100 33
    ```

* rmLog.sh: remove log file on remote machines.
  * `./rmLog.sh <the number of remote machines>`

### script/client
* deliverNode.sh: deliver client to remote machines.
  * `./deliverClient.sh <the number of remote machines>`

* cal.py: Calculate throughput and latency
  * usage: python3 cal.py <number of clients> <batchsize> <path of log directory> <name of log file> <test time>

* time_series.py: analyze real-time changes in throughput
  * usage: python3 time_series.py [-h] -n NUMBER -d DIRECTORY -f FILENAME -b BATCHSIZE -i INTERVAL

* Get log files from remote machines
    ```bash
    mkdir log
    ./createDir.sh <the number of remote machines>
    ./copyResult.sh <the number of remote machines> <name of log files>
    ```

## Note:
* Current scripts can be used by remote test in different machines. You can boost local test manually: modify the setting, start node and client processes, and analyze the result.
* Make port 5000-7000 open in the security group.
