package gem

import "testing"

func TestControlJobLifecycle(t *testing.T) {
	cjm := NewControlJobManager()

	err := cjm.Create("CJ-001", []string{"PJ-001", "PJ-002"}, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	job, ok := cjm.GetJob("CJ-001")
	if !ok {
		t.Fatal("job not found")
	}
	if job.State != CJQueued {
		t.Errorf("state = %s, want QUEUED", job.State)
	}

	// Full lifecycle: Queued -> Selected -> WaitingForStart -> Executing -> Completed
	for _, step := range []struct {
		fn    func(string) error
		state ControlJobState
	}{
		{cjm.Select, CJSelected},
		{cjm.WaitForStart, CJWaitingForStart},
		{cjm.Execute, CJExecuting},
		{cjm.Complete, CJCompleted},
	} {
		if err := step.fn("CJ-001"); err != nil {
			t.Fatalf("%s: %v", step.state, err)
		}
		job, _ = cjm.GetJob("CJ-001")
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
}

func TestControlJobPause(t *testing.T) {
	cjm := NewControlJobManager()
	cjm.Create("CJ-002", []string{"PJ-003"}, 1)
	cjm.Select("CJ-002")
	cjm.WaitForStart("CJ-002")
	cjm.Execute("CJ-002")

	cjm.Pause("CJ-002")
	cjm.PauseDone("CJ-002")
	job, _ := cjm.GetJob("CJ-002")
	if job.State != CJPaused {
		t.Errorf("state = %s, want PAUSED", job.State)
	}

	// Resume from paused
	cjm.Execute("CJ-002")
	job, _ = cjm.GetJob("CJ-002")
	if job.State != CJExecuting {
		t.Errorf("state = %s, want EXECUTING", job.State)
	}
}

func TestControlJobStop(t *testing.T) {
	cjm := NewControlJobManager()
	cjm.Create("CJ-003", []string{"PJ-004"}, 1)
	cjm.Select("CJ-003")
	cjm.WaitForStart("CJ-003")
	cjm.Execute("CJ-003")

	cjm.Stop("CJ-003")
	cjm.StopDone("CJ-003")
	job, _ := cjm.GetJob("CJ-003")
	if job.State != CJStopped {
		t.Errorf("state = %s, want STOPPED", job.State)
	}
}

func TestControlJobInvalidTransition(t *testing.T) {
	cjm := NewControlJobManager()
	cjm.Create("CJ-004", nil, 1)

	// Can't execute from Queued
	if err := cjm.Execute("CJ-004"); err == nil {
		t.Error("expected error for invalid transition")
	}
}

func TestControlJobRemove(t *testing.T) {
	cjm := NewControlJobManager()
	cjm.Create("CJ-005", nil, 1)

	// Can't remove active job
	if err := cjm.Remove("CJ-005"); err == nil {
		t.Error("expected error: cannot remove active job")
	}

	// Stop and remove
	cjm.Stop("CJ-005")
	cjm.StopDone("CJ-005")
	if err := cjm.Remove("CJ-005"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestControlJobCallback(t *testing.T) {
	cjm := NewControlJobManager()
	var gotOld, gotNew ControlJobState
	cjm.OnStateChange(func(id string, old, new_ ControlJobState) {
		gotOld = old
		gotNew = new_
	})

	cjm.Create("CJ-006", nil, 1)
	cjm.Select("CJ-006")

	if gotOld != CJQueued || gotNew != CJSelected {
		t.Errorf("transition = %s->%s, want QUEUED->SELECTED", gotOld, gotNew)
	}
}

func TestControlJobList(t *testing.T) {
	cjm := NewControlJobManager()
	cjm.Create("A", nil, 1)
	cjm.Create("B", nil, 2)

	jobs := cjm.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("job count = %d, want 2", len(jobs))
	}
}
