package io.trackline.streamproc;

public class FenceState {
    public long sinceMs;
    public boolean dwellEmitted;

    public FenceState() {
    }

    public FenceState(long sinceMs) {
        this.sinceMs = sinceMs;
    }
}
