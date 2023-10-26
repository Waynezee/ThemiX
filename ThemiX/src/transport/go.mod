module go.themix.io/transport

go 1.16

require (
	github.com/perlin-network/noise v1.1.3
	go.themix.io/client v0.0.0-00010101000000-000000000000
	go.themix.io/crypto v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.27.1
)

replace go.themix.io/themix => ../themix

replace go.themix.io/crypto => ../crypto

replace go.themix.io/client => ../client
