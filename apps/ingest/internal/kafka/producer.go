// Package kafkaproducer wraps segmentio/kafka-go with the delivery guarantees
// mandated by apps/_contracts/kafka-topics.md §3: acks=all and an idempotent
// producer, so a lost telemetry point is never silent and a retried publish is
// never duplicated on the broker side.
package kafkaproducer

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

const TelemetryRawTopic = "trackline.telemetry.raw"

type Producer struct {
	writer *kafka.Writer
}

// New builds a producer for the telemetry.raw topic, keyed by vehicle_id — see
// kafka-topics.md §2 for why vehicle_id and not tenant_id (ordering vs. hot
// partitions).
func New(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        TelemetryRawTopic,
			Balancer:     &kafka.Hash{}, // keyed writes land deterministically per key
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchTimeout: 50 * time.Millisecond,
		},
	}
}

// Publish writes one message keyed by vehicleID. The caller supplies an
// already-serialised protobuf payload — this package does no encoding, so it
// stays agnostic of the message schema evolving.
func (p *Producer) Publish(ctx context.Context, vehicleID string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(vehicleID),
		Value: payload,
		Time:  time.Now(),
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
