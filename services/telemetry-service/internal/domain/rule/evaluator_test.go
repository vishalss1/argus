package rule

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeRepo struct {
	Repository
	byWorkspace map[string][]Rule
	calls       int
}

func (f *fakeRepo) ListEnabledRulesByWorkspace(_ context.Context, workspaceID string) ([]Rule, error) {
	f.calls++
	return f.byWorkspace[workspaceID], nil
}

func TestMatches(t *testing.T) {
	cases := []struct {
		op       string
		observed float64
		want     bool
	}{
		{OperatorGreaterThan, 11, true}, {OperatorGreaterThan, 10, false},
		{OperatorGreaterThanOrEqual, 10, true}, {OperatorLessThan, 9, true},
		{OperatorLessThanOrEqual, 10, true}, {OperatorEqual, 10, true},
		{OperatorNotEqual, 10, false}, {"bogus", 10, false},
	}
	for _, c := range cases {
		if got := matches(c.op, c.observed, 10); got != c.want {
			t.Errorf("matches(%q, %v, 10) = %v, want %v", c.op, c.observed, got, c.want)
		}
	}
}

func TestEvaluateOnlyUsesOwnWorkspaceRules(t *testing.T) {
	repo := &fakeRepo{byWorkspace: map[string][]Rule{
		"ws-a": {{ID: "r1", Name: "hot", Metric: "temp", Operator: ">", Threshold: 50}},
		"ws-b": {{ID: "r2", Name: "cold", Metric: "temp", Operator: "<", Threshold: 0}},
	}}
	ev := NewEvaluator(repo, nil)
	metrics := json.RawMessage(`{"temp": 60, "label": "x"}`)

	alerts, err := ev.Evaluate(context.Background(), "ws-a", "dev1", "t1", metrics)
	if err != nil || len(alerts) != 1 || alerts[0].RuleID != "r1" || alerts[0].WorkspaceID != "ws-a" {
		t.Fatalf("ws-a: alerts=%+v err=%v", alerts, err)
	}

	alerts, err = ev.Evaluate(context.Background(), "ws-b", "dev2", "t2", metrics)
	if err != nil || len(alerts) != 0 {
		t.Fatalf("ws-b must not fire ws-a's rule: alerts=%+v err=%v", alerts, err)
	}

	if alerts, _ := ev.Evaluate(context.Background(), "", "dev3", "", metrics); len(alerts) != 0 {
		t.Fatalf("no workspace must yield no alerts, got %+v", alerts)
	}
}

func TestEvaluateCooldownAndCache(t *testing.T) {
	repo := &fakeRepo{byWorkspace: map[string][]Rule{
		"ws": {{ID: "r1", Name: "hot", Metric: "temp", Operator: ">", Threshold: 50}},
	}}
	allowed := false
	ev := NewEvaluator(repo, func(context.Context, string, string) bool { return allowed })
	metrics := json.RawMessage(`{"temp": 60}`)

	if alerts, _ := ev.Evaluate(context.Background(), "ws", "d", "", metrics); len(alerts) != 0 {
		t.Fatalf("cooldown should suppress alert, got %+v", alerts)
	}
	allowed = true
	alerts, _ := ev.Evaluate(context.Background(), "ws", "d", "", metrics)
	if len(alerts) != 1 || alerts[0].TelemetryID != nil {
		t.Fatalf("expected one alert with nil telemetry id, got %+v", alerts)
	}
	if repo.calls != 1 {
		t.Fatalf("rules should be cached, repo called %d times", repo.calls)
	}
}
