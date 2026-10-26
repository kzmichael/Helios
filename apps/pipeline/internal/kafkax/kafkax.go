// Package kafkax wraps segmentio/kafka-go with the delivery rules from
// apps/_contracts/kafka-topics.md: manual commit after processing, bounded retry
// with jittered backoff, and a dead-letter topic so one poison message never
// blocks a partition.
package kafkax

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	TopicTelemetryRaw      = "trackline.telemetry.raw"
	TopicEventsGeofence    = "trackline.events.geofence"
	TopicEventsViolation   = "trackline.events.violation"
	TopicEventsVehicle     = "trackline.events.vehicle"
	TopicAlertsRaised      = "trackline.alerts.raised"
	TopicNotificationsReq  = "trackline.notifications.requested"
	TopicNotificationsDone = "trackline.notifications.delivered"
	TopicUsageRecorded     = "trackline.usage.recorded"
	TopicTelemetryDLQ      = "trackline.telemetry.dlq"
	TopicNotificationsDLQ  = "trackline.notifications.dlq"

	// HeaderEventType tells consumers which message a shared topic carries
	// (trackline.events.violation holds both SpeedingEvent and IdleEvent).
	HeaderEventType = "x-event-type"

	maxAttempts = 5
)

type Message = kafka.Message

// Producer writes to any topic with acks=all.
type Producer struct{ w *kafka.Writer }

func NewProducer(brokers []string) *Producer {
	return &Producer{w: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		BatchTimeout:           20 * time.Millisecond,
		AllowAutoTopicCreation: true,
	}}
}

// Publish writes one keyed message. eventType, when set, becomes the x-event-type header.
func (p *Producer) Publish(ctx context.Context, topic, key, eventType string, value []byte) error {
	m := kafka.Message{Topic: topic, Key: []byte(key), Value: value, Time: time.Now()}
	if eventType != "" {
		m.Headers = []kafka.Header{{Key: HeaderEventType, Value: []byte(eventType)}}
	}
	return p.w.WriteMessages(ctx, m)
}

// PublishAll writes several messages in one round trip.
func (p *Producer) PublishAll(ctx context.Context, msgs []kafka.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	return p.w.WriteMessages(ctx, msgs...)
}

// DeadLetter parks a message that cannot be processed, recording why.
func (p *Producer) DeadLetter(ctx context.Context, topic string, original kafka.Message, reason string) error {
	return p.w.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   original.Key,
		Value: original.Value,
		Headers: append(original.Headers,
			kafka.Header{Key: "x-dlq-reason", Value: []byte(reason)},
			kafka.Header{Key: "x-dlq-source-topic", Value: []byte(original.Topic)},
		),
	})
}

func (p *Producer) Close() error { return p.w.Close() }

// Handler processes one batch. Return an error only for transient failures (a
// database outage, a broker timeout); permanent per-message problems belong in
// the handler, which should dead-letter them and carry on.
type Handler func(ctx context.Context, msgs []kafka.Message) error

type Options struct {
	Brokers []string
	GroupID string
	Topics  []string

	// StartOffset applies only when the group has no committed offset.
	StartOffset int64

	BatchSize int
	BatchWait time.Duration

	// DLQTopic receives a batch whose handler keeps failing. Empty drops it with a log line.
	DLQTopic string
	Producer *Producer
}

// Run consumes until ctx is cancelled. Offsets are committed only after the
// handler returns, so a crash redelivers rather than loses (at-least-once).
func Run(ctx context.Context, o Options, h Handler, log *slog.Logger) error {
	if o.BatchSize <= 0 {
		o.BatchSize = 500
	}
	if o.BatchWait <= 0 {
		o.BatchWait = 200 * time.Millisecond
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     o.Brokers,
		GroupID:     o.GroupID,
		GroupTopics: o.Topics,
		StartOffset: o.StartOffset,
		MinBytes:    1,
		MaxBytes:    10 << 20,
		MaxWait:     250 * time.Millisecond,
	})
	defer r.Close()

	log = log.With("group", o.GroupID)
	log.Info("consumer started", "topics", o.Topics)

	for ctx.Err() == nil {
		batch, err := fetchBatch(ctx, r, o.BatchSize, o.BatchWait)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Error("fetch failed", "error", err)
			sleep(ctx, time.Second)
			continue
		}
		if len(batch) == 0 {
			continue
		}

		herr := retry(ctx, func() error { return h(ctx, batch) })
		if herr != nil {
			if ctx.Err() != nil {
				break // shutting down mid-batch: leave it uncommitted for redelivery
			}
			if o.DLQTopic == "" {
				// Nowhere safe to park this batch (alerts, usage, delivery records), so the
				// only choice that loses nothing is to keep retrying until it succeeds.
				log.Error("batch failing, retrying until it succeeds", "messages", len(batch), "error", herr)
				for ctx.Err() == nil {
					sleep(ctx, 10*time.Second)
					if herr = h(ctx, batch); herr == nil {
						break
					}
				}
				if herr != nil {
					break
				}
			} else {
				log.Error("batch failed after retries, dead-lettering", "messages", len(batch), "error", herr)
				deadLetterBatch(o, batch, herr, log)
			}
		}

		commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if err := r.CommitMessages(commitCtx, batch...); err != nil {
			log.Error("commit failed", "error", err)
		}
		cancel()
	}
	log.Info("consumer stopped")
	return nil
}

func fetchBatch(ctx context.Context, r *kafka.Reader, size int, wait time.Duration) ([]kafka.Message, error) {
	first, err := r.FetchMessage(ctx)
	if err != nil {
		return nil, err
	}
	batch := []kafka.Message{first}

	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for len(batch) < size {
		m, err := r.FetchMessage(waitCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				break
			}
			return batch, err
		}
		batch = append(batch, m)
	}
	return batch, nil
}

func retry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if attempt == maxAttempts || ctx.Err() != nil {
			break
		}
		sleep(ctx, Backoff(attempt))
	}
	return fmt.Errorf("after %d attempts: %w", maxAttempts, err)
}

// Backoff is exponential with up to 30% jitter, so consumers recovering from the
// same outage do not all retry in lockstep.
func Backoff(attempt int) time.Duration {
	base := 500 * time.Millisecond << min(attempt-1, 5)
	if base > 10*time.Second {
		base = 10 * time.Second
	}
	return time.Duration(float64(base) * (1 + rand.Float64()*0.3))
}

func deadLetterBatch(o Options, batch []kafka.Message, cause error, log *slog.Logger) {
	if o.DLQTopic == "" || o.Producer == nil {
		log.Warn("no DLQ configured, dropping batch", "messages", len(batch))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, m := range batch {
		if err := o.Producer.DeadLetter(ctx, o.DLQTopic, m, "transient_retries_exhausted: "+cause.Error()); err != nil {
			log.Error("dead-letter failed", "error", err)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
