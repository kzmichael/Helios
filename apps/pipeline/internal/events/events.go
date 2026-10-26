// Package events builds the shared envelope for messages the pipeline publishes.
package events

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

// NewID returns a random RFC 4122 version 4 UUID.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return format(b)
}

// UUID5 derives a stable UUID from a name, so replays of the same logical event
// produce the same id and can be de-duplicated by a primary key.
func UUID5(name string) string {
	ns := [16]byte{0x74, 0x72, 0x61, 0x63, 0x6b, 0x6c, 0x69, 0x6e, 0x65, 0x2d, 0x76, 0x31, 0x2d, 0x69, 0x64, 0x73} // "trackline-v1-ids"
	h := sha1.New()
	h.Write(ns[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	return format(b)
}

func format(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// Meta builds an EventMeta. idempotencyKey must be stable across retries of the
// same logical event.
func Meta(tenantID, source, idempotencyKey string, occurredAt time.Time) *tracklinev1.EventMeta {
	return &tracklinev1.EventMeta{
		EventId:        NewID(),
		TenantId:       tenantID,
		Source:         source,
		OccurredAt:     occurredAt.UnixMilli(),
		IdempotencyKey: idempotencyKey,
	}
}
