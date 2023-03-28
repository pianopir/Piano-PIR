module easypir

go 1.20

replace example.com/query => ./query

replace example.com/util => ./util

require (
	example.com/query v0.0.0-00010101000000-000000000000
	example.com/util v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.53.0
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/holiman/uint256 v1.2.1 // indirect
	gitlab.com/yawning/chacha20.git v0.0.0-20190903091407-6d1cb28dc72c // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	google.golang.org/genproto v0.0.0-20230320184635-7606e756e683 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
