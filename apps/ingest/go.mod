module github.com/helios/trackline/apps/ingest

go 1.23

require (
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/helios/trackline/gen/go v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.6.1
	github.com/segmentio/kafka-go v0.4.47
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
)

// The gen/go module is produced by `buf generate` from apps/_proto (never
// committed — see docs/07_REPOSITORY_BLUEPRINT.md §13). Run buf generate before
// building this service.
replace github.com/helios/trackline/gen/go => ../gen/go
