package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"SyncHS-go/core/server"
	"SyncHS-go/crypto/ecdsa"
	"SyncHS-go/transport/http"
	"SyncHS-go/transport/info"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogger(id int, debug bool) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{
		"log/server" + strconv.Itoa(id),
	}
	cfg.Sampling = nil
	if debug {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	lg, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("NewLogger: %v", err)
	}
	return lg, nil
}

func removeLastRune(s string) string {
	r := []rune(s)
	return string(r[:len(r)-1])
}

type Configuration struct {
	Id      uint64 `json:"id"`
	Batch   int    `json:"batch"`
	Port    int    `json:"port"`
	Address string `json:"address"`
	Key     string `json:"key_path"`
	Cluster string `json:"cluster"`
	Pk      string `json:"pk"`
	Ck      string `json:"ck"`
}

var DEBUG = flag.Bool("debug", true, "enable debug logging")
var CONFIG_FILE = flag.String("conf", "node.json", "config file")

func main() {

	flag.Parse()
	jsonFile, err := os.Open(*CONFIG_FILE)
	if err != nil {
		panic(fmt.Sprint("os.Open: ", err))
	}
	defer jsonFile.Close()

	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		panic(fmt.Sprint("ioutil.ReadAll: ", err))
	}
	var config Configuration
	json.Unmarshal([]byte(data), &config)

	lg, err := newLogger(int(config.Id), *DEBUG)
	if err != nil {
		panic(fmt.Sprintf("newLogger: ", err))
	}
	defer lg.Sync()

	pk, err := ecdsa.LoadKey(config.Pk)
	if err != nil {
		panic(err)
	}
	addrs := strings.Split(config.Cluster, ",")
	var peers []http.Peer
	for i := 0; i < len(addrs); i++ {
		peer := http.Peer{
			PeerID:    uint32(i),
			PublicKey: pk,
			Addr:      addrs[i],
		}
		peers = append(peers, peer)
	}

	// bls, err := bls.InitBLS(config.Key, len(addrs), int(len(addrs)*3/4+1), int(config.Id))
	// if err != nil {
	// 	panic(fmt.Sprint("bls.InitBLS: ", err))
	// }

	n := server.InitNode(lg, info.IDType(config.Id), uint64(len(addrs)), config.Port, peers, pk)
	n.Run()

}
