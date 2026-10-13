//! Retry policy for notification delivery.
//!
//! Exponential backoff **with jitter** — jitter is not optional decoration. Without
//! it, every failed delivery for the same outage retries in lockstep, producing a
//! synchronized retry storm the moment the downstream recovers. See
//! docs/05_STUDY_GUIDE.md Phase 6 §"Resilience patterns".
use rand::Rng;
use std::time::Duration;

pub const MAX_ATTEMPTS: u32 = 5;

/// Backoff = base * 2^(attempt-1), capped, plus up to 30% random jitter.
pub fn backoff_for_attempt(attempt: u32) -> Duration {
    let base_ms: u64 = 500;
    let cap_ms: u64 = 30_000;

    let exp = base_ms.saturating_mul(1u64 << attempt.min(6));
    let capped = exp.min(cap_ms);

    let mut rng = rand::thread_rng();
    let jitter_frac: f64 = rng.gen_range(0.0..0.3);
    let jittered = (capped as f64) * (1.0 + jitter_frac);

    Duration::from_millis(jittered as u64)
}

pub fn should_give_up(attempt: u32) -> bool {
    attempt >= MAX_ATTEMPTS
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn backoff_grows_with_attempt() {
        assert!(backoff_for_attempt(1) < backoff_for_attempt(4));
    }

    #[test]
    fn gives_up_after_max_attempts() {
        assert!(!should_give_up(MAX_ATTEMPTS - 1));
        assert!(should_give_up(MAX_ATTEMPTS));
    }
}
