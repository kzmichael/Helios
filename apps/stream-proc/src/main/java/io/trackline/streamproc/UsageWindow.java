package io.trackline.streamproc;

import java.util.HashSet;
import java.util.Set;
import org.apache.flink.api.common.functions.AggregateFunction;
import org.apache.flink.streaming.api.functions.windowing.ProcessWindowFunction;
import org.apache.flink.streaming.api.windowing.windows.TimeWindow;
import org.apache.flink.util.Collector;

/**
 * Per-tenant usage over tumbling event-time windows (the window's clock is the
 * time
 * the point reached ingest, so a device with a wrong clock cannot move usage
 * into
 * another day). Each closed window becomes one UsageRecorded delta.
 */
public final class UsageWindow {
    private UsageWindow() {
    }

    public static class Acc {
        public long points;
        public Set<String> devices = new HashSet<>();

        public Acc() {
        }
    }

    public static class Count implements AggregateFunction<Observation, Acc, Acc> {
        @Override
        public Acc createAccumulator() {
            return new Acc();
        }

        @Override
        public Acc add(Observation o, Acc acc) {
            acc.points++;
            acc.devices.add(o.deviceId);
            return acc;
        }

        @Override
        public Acc getResult(Acc acc) {
            return acc;
        }

        @Override
        public Acc merge(Acc a, Acc b) {
            a.points += b.points;
            a.devices.addAll(b.devices);
            return a;
        }
    }

    public static class Emit extends ProcessWindowFunction<Acc, OutRecord, String, TimeWindow> {
        @Override
        public void process(String tenantId, Context ctx, Iterable<Acc> results, Collector<OutRecord> out) {
            Acc acc = results.iterator().next();
            out.collect(OutRecord.usage(tenantId, ctx.window().getStart(), ctx.window().getEnd(), acc.points,
                    acc.devices.size()));
        }
    }
}
