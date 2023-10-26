package transport

import (
	"crypto/ecdsa"

	"go.themix.io/transport/http"
	"go.themix.io/transport/proto/consmsgpb"
	"go.uber.org/zap"
)

type Transport interface {
	Broadcast(msg *consmsgpb.WholeMessage)
	SendMessage(id uint32, msg *consmsgpb.WholeMessage)
}

// InitTransport executes transport layer initiliazation, which returns transport, a channel
// for received ConsMessage, a channel for received requests, and a channel for reply
func InitTransport(lg *zap.Logger, id uint32, port int, peers []http.Peer, ck *ecdsa.PrivateKey, sign bool, batchsize int) (Transport,
	chan *consmsgpb.WholeMessage, chan []byte, chan []byte) {
	return http.InitTransport(lg, id, port, peers, ck, sign, batchsize)
}
