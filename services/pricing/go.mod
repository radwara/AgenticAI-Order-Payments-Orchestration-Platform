module github.com/radwara/agentic-order-platform/services/pricing

go 1.26.0

require (
	github.com/radwara/agentic-order-platform/libs/go-money v0.0.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)

replace github.com/radwara/agentic-order-platform/libs/go-money => ../../libs/go-money
