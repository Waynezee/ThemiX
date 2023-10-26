module SyncHS-go/core

go 1.19

require (
	SyncHS-go/crypto v0.0.0-00010101000000-000000000000
	SyncHS-go/transport v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/oasislabs/ed25519 v0.0.0-20200302143042-29f6767a7c3e // indirect
	github.com/perlin-network/noise v1.1.3 // indirect
	go.dedis.ch/fixbuf v1.0.3 // indirect
	go.dedis.ch/kyber/v3 v3.0.13 // indirect
	go.uber.org/atomic v1.6.0 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	golang.org/x/crypto v0.0.0-20191119213627-4f8c1d86b1ba // indirect
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150 // indirect
)

replace SyncHS-go/transport => ../transport

replace SyncHS-go/crypto => ../crypto

replace SyncHS-go/crypto/sha256 => ../crypto/sha256
