package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"go.themix.io/crypto/bls"
	myecdsa "go.themix.io/crypto/ecdsa"
	"go.themix.io/themix/server"
	"go.themix.io/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogger(id int) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		"log/server" + strconv.Itoa(id),
	}
	cfg.Sampling = nil
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	return cfg.Build()
}

func removeLastRune(s string) string {
	r := []rune(s)
	return string(r[:len(r)-1])
}

type Configuration struct {
	Id      uint64 `json:"id"`
	Batch   int    `json:"batchsize"`
	Port    int    `json:"port"`
	Address string `json:"adress"`
	Key     string `json:"key_path"`
	Cluster string `json:"cluster"`
	Pk      string `json:"pk"`
	Ck      string `json:"ck"`
}

func main() {
	sign := flag.Bool("sign", false, "whether to verify client sign or not")
	batchsize := flag.Int("batch", 1, "how many times for a client signature being verified")
	flag.Parse()

	jsonFile, err := os.Open("node.json")
	if err != nil {
		panic(fmt.Sprint("os.Open: ", err))
	}
	defer jsonFile.Close()

	data, _ := ioutil.ReadAll(jsonFile)
	var config Configuration
	json.Unmarshal([]byte(data), &config)

	lg, err := newLogger(int(config.Id))
	if err != nil {
		panic(fmt.Sprint("newLogger: ", err))
	}
	defer lg.Sync()

	addrs := strings.Split(config.Cluster, ",")
	fmt.Printf("%d %s %d\n", config.Id, addrs, len(addrs))
	bls, err := bls.InitBLS(config.Key, len(addrs), int(len(addrs)/2+1), int(config.Id))
	if err != nil {
		panic(fmt.Sprint("bls.InitBLS: ", err))
	}
	pk, _ := myecdsa.LoadKey(config.Pk)
	var peers []http.Peer
	for i := 0; i < len(addrs); i++ {
		peer := http.Peer{
			PeerID:    uint32(i),
			PublicKey: pk,
			Addr:      addrs[i],
		}
		peers = append(peers, peer)
	}
	ckPath := config.Ck
	ck, err := myecdsa.LoadKey(ckPath)
	if err != nil {
		panic(fmt.Sprintf("ecdsa.LoadKey: %v", err))
	}
	server.InitNode(lg, bls, config.Pk, uint32(config.Id), uint64(len(addrs)), config.Port, peers, *batchsize, ck, *sign)
}
