module github.com/helios/trackline/apps/device-sim

go 1.23

require (
	github.com/helios/trackline/gen/go v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.65.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/helios/trackline/gen/go => ../gen/go
