package gem

import "testing"

func TestProcessJobLifecycle(t *testing.T) {
	pm := NewProcessJobManager()

	err := pm.Create("PJ-001", "RECIPE-A", "FOUP-001", []int{1, 2, 3}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	job, ok := pm.GetJob("PJ-001")
	if !ok {
		t.Fatal("job not found")
	}
	if job.State != PJQueued {
		t.Errorf("state = %s, want QUEUED", job.State)
	}

	// Full lifecycle: Queued -> SettingUp -> WaitingForStart -> Processing -> ProcessComplete
	for _, step := range []struct {
		fn    func(string) error
		state ProcessJobState
	}{
		{pm.Setup, PJSettingUp},
		{pm.SetupComplete, PJWaitingForStart},
		{pm.Start, PJProcessing},
		{pm.Complete, PJProcessComplete},
	} {
		if err := step.fn("PJ-001"); err != nil {
			t.Fatalf("%s: %v", step.state, err)
		}
		job, _ = pm.GetJob("PJ-001")
		if job.State != step.state {
			t.Errorf("state = %s, want %s", job.State, step.state)
		}
	}

	if job.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if job.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}

	// Remove completed job
	if err := pm.Remove("PJ-001"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestProcessJobAbort(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("PJ-002", "RECIPE-B", "FOUP-002", []int{1}, nil)
	pm.Setup("PJ-002")
	pm.SetupComplete("PJ-002")
	pm.Start("PJ-002")

	if err := pm.Abort("PJ-002"); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	job, _ := pm.GetJob("PJ-002")
	if job.State != PJAborting {
		t.Errorf("state = %s, want ABORTING", job.State)
	}

	if err := pm.AbortDone("PJ-002"); err != nil {
		t.Fatalf("AbortDone: %v", err)
	}
	job, _ = pm.GetJob("PJ-002")
	if job.State != PJAborted {
		t.Errorf("state = %s, want ABORTED", job.State)
	}
}

func TestProcessJobStop(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("PJ-003", "RECIPE-C", "FOUP-003", []int{1, 2}, nil)
	pm.Setup("PJ-003")
	pm.SetupComplete("PJ-003")
	pm.Start("PJ-003")

	pm.Stop("PJ-003")
	pm.StopDone("PJ-003")

	job, _ := pm.GetJob("PJ-003")
	if job.State != PJStopped {
		t.Errorf("state = %s, want STOPPED", job.State)
	}
}

func TestProcessJobInvalidTransition(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("PJ-004", "RECIPE-D", "FOUP-004", nil, nil)

	// Can't start from Queued (must setup first)
	if err := pm.Start("PJ-004"); err == nil {
		t.Error("expected error for invalid transition")
	}

	// Can't complete from Queued
	if err := pm.Complete("PJ-004"); err == nil {
		t.Error("expected error for invalid transition")
	}
}

func TestProcessJobDuplicate(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("PJ-005", "R", "C", nil, nil)
	if err := pm.Create("PJ-005", "R", "C", nil, nil); err == nil {
		t.Error("expected error for duplicate job")
	}
}

func TestProcessJobRemoveActive(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("PJ-006", "R", "C", nil, nil)
	if err := pm.Remove("PJ-006"); err == nil {
		t.Error("expected error: cannot remove active job")
	}
}

func TestProcessJobCallback(t *testing.T) {
	pm := NewProcessJobManager()
	var gotOld, gotNew ProcessJobState
	pm.OnStateChange(func(id string, old, new_ ProcessJobState) {
		gotOld = old
		gotNew = new_
	})

	pm.Create("PJ-007", "R", "C", nil, nil)
	pm.Setup("PJ-007")

	if gotOld != PJQueued || gotNew != PJSettingUp {
		t.Errorf("transition = %s->%s, want QUEUED->SETTING_UP", gotOld, gotNew)
	}
}

func TestProcessJobListActive(t *testing.T) {
	pm := NewProcessJobManager()
	pm.Create("A", "R", "C", nil, nil)
	pm.Create("B", "R", "C", nil, nil)
	pm.Create("C", "R", "C", nil, nil)

	// Complete one
	pm.Setup("A")
	pm.SetupComplete("A")
	pm.Start("A")
	pm.Complete("A")

	active := pm.ListActiveJobs()
	if len(active) != 2 {
		t.Errorf("active jobs = %d, want 2", len(active))
	}

	all := pm.ListJobs()
	if len(all) != 3 {
		t.Errorf("all jobs = %d, want 3", len(all))
	}
}
