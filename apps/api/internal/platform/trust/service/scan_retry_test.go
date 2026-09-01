package service

import (
	"context"
	"errors"
	"testing"

	"api/internal/platform/trust/model"
)

type scriptedGateway struct {
	steps []scriptStep
	calls int
}

type scriptStep struct {
	verdict GatewayVerdict
	err     error
}

func (g *scriptedGateway) Configured() bool { return true }

func (g *scriptedGateway) Moderate(_ context.Context, _, _, _ string, _ *int64) (GatewayVerdict, error) {
	s := g.steps[len(g.steps)-1]
	if g.calls < len(g.steps) {
		s = g.steps[g.calls]
	}
	g.calls++
	return s.verdict, s.err
}

func TestScanDegradedReasonRecorded(t *testing.T) {
	cases := []struct {
		name    string
		gateway ScanGateway
		want    int16
	}{
		{"unconfigured", &fakeGateway{configured: false}, model.ScanDegradedGatewayUnconfigured},
		{"call-failed", &fakeGateway{configured: true, err: errors.New("upstream boom")}, model.ScanDegradedGatewayCallFailed},
		{"gateway-degraded", &fakeGateway{configured: true, verdict: GatewayVerdict{Degraded: true}}, model.ScanDegradedGatewayDegraded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cleanTables(t)
			id := seedPending(t, "s-reason", "text")
			w := NewScanWorker(testDB, c.gateway, stubTier0{})
			if _, err := w.ScorePending(context.Background()); err != nil {
				t.Fatalf("score: %v", err)
			}
			row := getScan(t, id)
			if row.Status != model.ScanStatusDegraded {
				t.Fatalf("status = %d, want degraded", row.Status)
			}
			if row.DegradedReason == nil || *row.DegradedReason != c.want {
				t.Fatalf("degraded_reason = %v, want %d", row.DegradedReason, c.want)
			}
			if row.ScanAttempts != 1 {
				t.Fatalf("scan_attempts = %d, want 1", row.ScanAttempts)
			}
		})
	}
}

func TestScanDegradedRetriedThenScored(t *testing.T) {
	cleanTables(t)
	id := seedPending(t, "s-recover", "text")

	g := &scriptedGateway{steps: []scriptStep{
		{err: errors.New("transient 503")},
		{verdict: GatewayVerdict{Flagged: true, Score: f32(0.8), Channel: "glm"}},
	}}
	w := NewScanWorker(testDB, g, stubTier0{})

	if _, err := w.ScorePending(context.Background()); err != nil {
		t.Fatalf("pass 1: %v", err)
	}
	if row := getScan(t, id); row.Status != model.ScanStatusDegraded {
		t.Fatalf("after pass 1 status = %d, want degraded", row.Status)
	}

	if _, err := w.ScorePending(context.Background()); err != nil {
		t.Fatalf("pass 2: %v", err)
	}
	row := getScan(t, id)
	if row.Status != model.ScanStatusScored {
		t.Fatalf("after pass 2 status = %d, want scored — the retry never happened", row.Status)
	}
	if row.DegradedReason != nil {
		t.Fatalf("a scored row must not keep a degradation reason, got %d", *row.DegradedReason)
	}
	if row.ScanAttempts != 2 {
		t.Fatalf("scan_attempts = %d, want 2 (the row took two tries and must say so)", row.ScanAttempts)
	}
	if row.Flagged == nil || !*row.Flagged {
		t.Fatal("the recovered verdict must be recorded")
	}
}

func TestScanDegradedRetryIsBounded(t *testing.T) {
	cleanTables(t)
	id := seedPending(t, "s-exhaust", "text")

	g := &fakeGateway{configured: true, err: errors.New("permanently down")}
	w := NewScanWorker(testDB, g, stubTier0{})

	for pass := range maxScanAttempts + 3 {
		if _, err := w.ScorePending(context.Background()); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
	}
	row := getScan(t, id)
	if row.Status != model.ScanStatusDegraded {
		t.Fatalf("status = %d, want degraded", row.Status)
	}
	if row.ScanAttempts != maxScanAttempts {
		t.Fatalf("scan_attempts = %d, want it capped at %d", row.ScanAttempts, maxScanAttempts)
	}
	if int(g.calls.Load()) != maxScanAttempts {
		t.Fatalf("gateway dialed %d times, want exactly %d — the bound is not holding",
			g.calls.Load(), maxScanAttempts)
	}
}

func TestScanPendingNotStarvedByRetries(t *testing.T) {
	cleanTables(t)

	failing := &fakeGateway{configured: true, err: errors.New("down")}
	wFail := NewScanWorker(testDB, failing, stubTier0{})
	for i := range scanBatchSize {
		seedPending(t, "s-old-"+string(rune('a'+i%26))+string(rune('0'+i/26)), "text")
	}
	if _, err := wFail.ScorePending(context.Background()); err != nil {
		t.Fatalf("seed drain: %v", err)
	}
	if got := countScanStatus(t, model.ScanStatusDegraded); got != int64(scanBatchSize) {
		t.Fatalf("expected %d degraded rows to retry against, got %d", scanBatchSize, got)
	}

	fresh := seedPending(t, "s-fresh", "text")
	good := &fakeGateway{configured: true, verdict: GatewayVerdict{Flagged: false, Score: f32(0.1), Channel: "omni"}}
	if _, err := NewScanWorker(testDB, good, stubTier0{}).ScorePending(context.Background()); err != nil {
		t.Fatalf("mixed pass: %v", err)
	}
	if row := getScan(t, fresh); row.Status != model.ScanStatusScored {
		t.Fatalf("fresh intake status = %d, want scored — retries starved the live queue", row.Status)
	}
}
