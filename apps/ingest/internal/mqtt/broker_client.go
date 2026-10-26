// Package mqtt is the fallback transport for constrained devices that cannot hold
// a gRPC stream open. Devices publish a protobuf PublishBatchRequest to
// trackline/devices/<device id>/telemetry on an MQTT broker; ingest subscribes
// and feeds the same pipeline gRPC uses.
//
// Authentication is the broker's job: its ACL must let a device publish only to
// its own topic. Ingest therefore trusts the device id in the topic, never the
// one in the payload.
package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// Topic filter ingest subscribes to; + is the device id.
const TopicFilter = "trackline/devices/+/telemetry"

// Ingester is what the transport feeds; grpcserver.Server implements it.
type Ingester interface {
	IngestBatch(ctx context.Context, deviceID, batchID string, points []*tracklinev1.TelemetryPoint) (accepted uint32, duplicate bool)
}

type Config struct {
	BrokerURL string // e.g. tcp://mosquitto:1883
	ClientID  string
	Username  string
	Password  string

	// SharedGroup, when set, subscribes as $share/<group>/... so several ingest
	// replicas split the stream (needs a broker with shared-subscription support).
	SharedGroup string
}

// DeviceIDFromTopic extracts <device id> from trackline/devices/<device id>/telemetry.
func DeviceIDFromTopic(topic string) (string, bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "trackline" || parts[1] != "devices" || parts[3] != "telemetry" || parts[2] == "" {
		return "", false
	}
	return parts[2], true
}

// Run connects, subscribes, and blocks until ctx is cancelled.
func Run(ctx context.Context, cfg Config, ing Ingester, logger *slog.Logger) error {
	filter := TopicFilter
	if cfg.SharedGroup != "" {
		filter = "$share/" + cfg.SharedGroup + "/" + TopicFilter
	}

	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetCleanSession(false).
		SetOrderMatters(false)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	handler := func(_ paho.Client, m paho.Message) {
		deviceID, ok := DeviceIDFromTopic(m.Topic())
		if !ok {
			return
		}
		var req tracklinev1.PublishBatchRequest
		if err := proto.Unmarshal(m.Payload(), &req); err != nil {
			logger.Warn("mqtt: undecodable payload", "device_id", deviceID, "error", err)
			return
		}
		accepted, dup := ing.IngestBatch(ctx, deviceID, req.GetBatchId(), req.GetPoints())
		logger.Debug("mqtt batch", "device_id", deviceID, "accepted", accepted, "duplicate", dup)
	}

	// Subscribe on every (re)connect: a clean broker session would otherwise forget us.
	opts.SetOnConnectHandler(func(c paho.Client) {
		if t := c.Subscribe(filter, 1, handler); t.Wait() && t.Error() != nil {
			logger.Error("mqtt: subscribe failed", "filter", filter, "error", t.Error())
			return
		}
		logger.Info("mqtt subscribed", "filter", filter)
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		logger.Warn("mqtt connection lost", "error", err)
	})

	client := paho.NewClient(opts)
	if t := client.Connect(); !t.WaitTimeout(10*time.Second) && ctx.Err() == nil {
		logger.Warn("mqtt: broker not reachable yet, will keep retrying", "broker", cfg.BrokerURL)
	} else if err := t.Error(); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}

	<-ctx.Done()
	client.Disconnect(500)
	return nil
}
