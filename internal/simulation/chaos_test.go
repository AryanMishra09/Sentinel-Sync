package simulation

import (
	"testing"
	"time"
)

func TestChaosDefaultNoDrop(t *testing.T) {
	c := NewChaos()
	for range 1000 {
		if c.ShouldDrop() {
			t.Fatal("default chaos should never drop")
		}
	}
}

func TestChaosIsolationAlwaysDrops(t *testing.T) {
	c := NewChaos()
	c.SetIsolated(true)
	for range 100 {
		if !c.ShouldDrop() {
			t.Fatal("isolated chaos must always drop")
		}
	}
	if !c.IsIsolated() {
		t.Fatal("IsIsolated should return true")
	}
}

func TestChaosIsolationLifted(t *testing.T) {
	c := NewChaos()
	c.SetIsolated(true)
	c.SetIsolated(false)
	if c.IsIsolated() {
		t.Fatal("isolation should be cleared")
	}
	// After lifting, ShouldDrop reverts to no-drop (loss is still 0).
	for range 100 {
		if c.ShouldDrop() {
			t.Fatal("lifted isolation + zero loss must not drop")
		}
	}
}

func TestChaosFullLossDropsAll(t *testing.T) {
	c := NewChaos()
	c.SetLossRate(1.0)
	for range 200 {
		if !c.ShouldDrop() {
			t.Fatal("100% loss must drop every message")
		}
	}
}

func TestChaosZeroLossDropsNone(t *testing.T) {
	c := NewChaos()
	c.SetLossRate(0.0)
	for range 1000 {
		if c.ShouldDrop() {
			t.Fatal("0% loss must not drop")
		}
	}
}

func TestChaosPartialLossApproximate(t *testing.T) {
	c := NewChaos()
	c.SetLossRate(0.5)
	const trials = 10_000
	dropped := 0
	for range trials {
		if c.ShouldDrop() {
			dropped++
		}
	}
	// 50% ± 5% with 10k trials is extremely conservative.
	ratio := float64(dropped) / trials
	if ratio < 0.45 || ratio > 0.55 {
		t.Fatalf("50%% loss: expected ~0.50 drop ratio, got %.3f", ratio)
	}
}

func TestChaosApplyDelayZero(t *testing.T) {
	c := NewChaos()
	start := time.Now()
	c.ApplyDelay()
	if elapsed := time.Since(start); elapsed > 5*time.Millisecond {
		t.Fatalf("zero latency delayed %v", elapsed)
	}
}

func TestChaosApplyDelayNonZero(t *testing.T) {
	c := NewChaos()
	c.SetLatency(20 * time.Millisecond)
	start := time.Now()
	c.ApplyDelay()
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("20ms latency only delayed %v", elapsed)
	}
}

func TestChaosSnapshot(t *testing.T) {
	c := NewChaos()
	c.SetLatency(100 * time.Millisecond)
	c.SetLossRate(0.25)
	c.SetIsolated(true)

	snap := c.Snapshot()
	if snap.LatencyMs != 100 {
		t.Fatalf("expected latencyMs=100, got %d", snap.LatencyMs)
	}
	if snap.LossRate != 0.25 {
		t.Fatalf("expected lossRate=0.25, got %f", snap.LossRate)
	}
	if !snap.Isolated {
		t.Fatal("expected isolated=true")
	}
}
