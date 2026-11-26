#!/bin/sh

set -eu

BOOTSTRAP="${KAFKA_BOOTSTRAP:-kafka:9092}"
BIN=/opt/kafka/bin

TOPICS="
trackline.telemetry.raw            86400000
trackline.events.geofence          604800000
trackline.events.violation         604800000
trackline.events.vehicle           604800000
trackline.alerts.raised            2592000000
trackline.notifications.requested  604800000
trackline.notifications.delivered  604800000
trackline.usage.recorded           7776000000
trackline.telemetry.dlq            2592000000
trackline.notifications.dlq        2592000000
"

echo "$TOPICS" | while read -r name retention; do
  if [ -z "$name" ]; then continue; fi
  "$BIN/kafka-topics.sh" --bootstrap-server "$BOOTSTRAP" --create --if-not-exists \
  --topic "$name" --partitions 1 --replication-factor 1 \
  --config "retention.ms=$retention"
  "$BIN/kafka-configs.sh" --bootstrap-server "$BOOTSTRAP" --alter \
  --entity-type topics --entity-name "$name" \
  --add-config "retention.ms=$retention"
done

"$BIN/kafka-topics.sh" --bootstrap-server "$BOOTSTRAP" --list

