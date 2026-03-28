package gem

import (
	"fmt"
	"sync"
	"time"
)

// --- E94 Control Job State Model ---

// ControlJobState represents the control job lifecycle per SEMI E94.
type ControlJobState int

const (
	CJQueued          ControlJobState = iota // Waiting to be selected
	CJSelected                               // Selected for execution
	CJWaitingForStart                        // Ready, awaiting trigger
	CJExecuting                              // Actively executing process jobs
	CJPausing                                // Graceful pause in progress
	CJPaused                                 // Paused
	CJCompleted                              // All process jobs done
	CJStopping                               // Graceful stop in progress
	CJStopped                                // Stopped before completion
)

func (s ControlJobState) String() string {
	switch s {
	case CJQueued:
		return "QUEUED"
	case CJSelected:
		return "SELECTED"
	case CJWaitingForStart:
		return "WAITING_FOR_START"
	case CJExecuting:
		return "EXECUTING"
	case CJPausing:
		return "PAUSING"
	case CJPaused:
		return "PAUSED"
	case CJCompleted:
		return "COMPLETED"
	case CJStopping:
		return "STOPPING"
	case CJStopped:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}

// ControlJob represents a control job per SEMI E94.
// A control job groups one or more process jobs for coordinated execution.
type ControlJob struct {
	JobID       string
	State       ControlJobState
	ProcessJobs []string          // IDs of child process jobs (E40)
	Priority    int               // Scheduling priority
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

// ControlJobManager manages control jobs per SEMI E94.
type ControlJobManager struct {
	mu       sync.RWMutex
	jobs     map[string]*ControlJob
	onChange func(jobID string, oldState, newState ControlJobState)
}

// NewControlJobManager creates an E94 control job manager.
func NewControlJobManager() *ControlJobManager {
	return &ControlJobManager{
		jobs: make(map[string]*ControlJob),
	}
}

// OnStateChange sets a callback for control job state transitions.
func (cjm *ControlJobManager) OnStateChange(fn func(jobID string, oldState, newState ControlJobState)) {
	cjm.mu.Lock()
	cjm.onChange = fn
	cjm.mu.Unlock()
}

// Create adds a new control job in Queued state.
func (cjm *ControlJobManager) Create(jobID string, processJobIDs []string, priority int) error {
	cjm.mu.Lock()
	defer cjm.mu.Unlock()

	if _, ok := cjm.jobs[jobID]; ok {
		return fmt.Errorf("e94: control job %s already exists", jobID)
	}

	cjm.jobs[jobID] = &ControlJob{
		JobID:       jobID,
		State:       CJQueued,
		ProcessJobs: processJobIDs,
		Priority:    priority,
		CreatedAt:   time.Now(),
	}
	return nil
}

func (cjm *ControlJobManager) transition(jobID string, expected []ControlJobState, newState ControlJobState) error {
	cjm.mu.Lock()
	defer cjm.mu.Unlock()

	job, ok := cjm.jobs[jobID]
	if !ok {
		return fmt.Errorf("e94: unknown control job %s", jobID)
	}

	valid := false
	for _, s := range expected {
		if job.State == s {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("e94: job %s cannot transition from %s to %s", jobID, job.State, newState)
	}

	oldState := job.State
	job.State = newState

	switch newState {
	case CJExecuting:
		job.StartedAt = time.Now()
	case CJCompleted, CJStopped:
		job.CompletedAt = time.Now()
	}

	if cjm.onChange != nil {
		cjm.onChange(jobID, oldState, newState)
	}
	return nil
}

// Select transitions from Queued to Selected.
func (cjm *ControlJobManager) Select(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJQueued}, CJSelected)
}

// WaitForStart transitions from Selected to WaitingForStart.
func (cjm *ControlJobManager) WaitForStart(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJSelected}, CJWaitingForStart)
}

// Execute transitions from WaitingForStart to Executing.
func (cjm *ControlJobManager) Execute(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJWaitingForStart, CJPaused}, CJExecuting)
}

// Complete transitions from Executing to Completed.
func (cjm *ControlJobManager) Complete(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJExecuting}, CJCompleted)
}

// Pause initiates graceful pause from Executing.
func (cjm *ControlJobManager) Pause(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJExecuting}, CJPausing)
}

// PauseDone transitions from Pausing to Paused.
func (cjm *ControlJobManager) PauseDone(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJPausing}, CJPaused)
}

// Stop initiates graceful stop.
func (cjm *ControlJobManager) Stop(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{
		CJQueued, CJSelected, CJWaitingForStart, CJExecuting, CJPausing, CJPaused,
	}, CJStopping)
}

// StopDone transitions from Stopping to Stopped.
func (cjm *ControlJobManager) StopDone(jobID string) error {
	return cjm.transition(jobID, []ControlJobState{CJStopping}, CJStopped)
}

// Remove deletes a completed/stopped control job.
func (cjm *ControlJobManager) Remove(jobID string) error {
	cjm.mu.Lock()
	defer cjm.mu.Unlock()
	job, ok := cjm.jobs[jobID]
	if !ok {
		return fmt.Errorf("e94: unknown control job %s", jobID)
	}
	if job.State != CJCompleted && job.State != CJStopped {
		return fmt.Errorf("e94: job %s not in terminal state (state=%s)", jobID, job.State)
	}
	delete(cjm.jobs, jobID)
	return nil
}

// GetJob returns a control job by ID.
func (cjm *ControlJobManager) GetJob(jobID string) (*ControlJob, bool) {
	cjm.mu.RLock()
	defer cjm.mu.RUnlock()
	j, ok := cjm.jobs[jobID]
	return j, ok
}

// ListJobs returns all control jobs.
func (cjm *ControlJobManager) ListJobs() []*ControlJob {
	cjm.mu.RLock()
	defer cjm.mu.RUnlock()
	result := make([]*ControlJob, 0, len(cjm.jobs))
	for _, j := range cjm.jobs {
		result = append(result, j)
	}
	return result
}
