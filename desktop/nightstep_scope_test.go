package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// B1: the night job's one step carries its scope, kept with the run; a night run a restart finds is let go, never
// hired (unscoped or not).
func TestANightJobsStepKeepsItsScopeAndAKeptNightRunIsNeverHired(t *testing.T) {
	turnOn(t, &nightShiftOn)
	j := NightJob{ID: "n1", Role: "janitor", Folder: "/p", Scope: "/p", ScopeRoles: "janitor"}
	step := j.nightStep()
	if step.Scope != "/p" || step.ScopeRoles != "janitor" {
		t.Fatalf("the night job's step: %+v", step)
	}
	file := filepath.Join(t.TempDir(), "hire-queue.json")
	running := step
	running.State = hqHiring
	data, _ := json.Marshal(hireQueueFile{Run: &hireQueueRun{ID: "r1", Label: "night job n1", Steps: []hireStep{running},
		State: hqRunning, Night: "n1"}})
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	hired := 0
	q := newHireQueue(file, hireQueueDeps{hireRole: func(hireStep) (int, string) { hired++; return http.StatusNoContent, "" }})
	snap := q.snapshot()["run"].(hireQueueRun)
	if snap.State != hqAbandoned || hired != 0 {
		t.Fatalf("a kept night run: %s, hired %d", snap.State, hired)
	}
	var kept hireQueueFile
	data, _ = os.ReadFile(file)
	if json.Unmarshal(data, &kept) != nil || kept.Run == nil || kept.Run.Steps[0].Scope != "/p" || kept.Run.Steps[0].ScopeRoles != "janitor" {
		t.Fatalf("the kept run lost its scope: %s", data)
	}
	if q.resume(actorPage) == nil {
		t.Fatal("a let-go night run was resumed")
	}
}
