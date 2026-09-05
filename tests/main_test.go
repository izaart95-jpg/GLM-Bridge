package tests

import (
    "os"
    "testing"
)

// Upstream pacing (UPSTREAM_MIN_INTERVAL_MS, see internal/zbridge/ratelimit.go)
// exists to keep bursts away from chat.z.ai. These tests talk to a local mock,
// which needs no protection and would otherwise pay the delay on every call —
// it stretched this suite from ~4 s to ~17 s.
func TestMain(m *testing.M) {
    if _, set := os.LookupEnv("UPSTREAM_MIN_INTERVAL_MS"); !set {
        os.Setenv("UPSTREAM_MIN_INTERVAL_MS", "0")
    }
    os.Exit(m.Run())
}
