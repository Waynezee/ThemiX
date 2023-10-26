package transport

import (
	"crypto/ecdsa"
	"fmt"

	"SyncHS-go/transport/http"
	"SyncHS-go/transport/info"
	"SyncHS-go/transport/message"

	"go.uber.org/zap"
)

type Transport interface {
	Broadcast(msg *message.ConsMessage)
}

// InitTransport executes transport layer initiliazation, which returns transport, a channel
// for received ConsMessage, a channel for received requests, and a channel for reply
func InitTransport(lg *zap.Logger, id info.IDType, port int, peers []http.Peer, pk *ecdsa.PrivateKey) (Transport,
	chan *message.ConsMessage, chan []byte, chan []byte) {
	fmt.Println("client prot:", port)
	return http.InitTransport(lg, id, port, peers, pk)
}
