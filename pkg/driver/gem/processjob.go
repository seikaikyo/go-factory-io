package gem

import (
	"fmt"
	"sync"
	"time"
)

// --- E40 Process Job State Model ---

// ProcessJobState represents the process job lifecycle per SEMI E40.
type ProcessJobState int

const (
	PJQueued           ProcessJobState = iota // Waiting to start
	PJSettingUp                               // Preparing resources
	PJWaitingForStart                         // Setup done, awaiting trigger
	PJProcessing                              // Actively processing
	PJProcessComplete                         // Processing finished
	PJStopping                                // Graceful stop in progress
	PJStopped                                 // Stopped before completion
	PJAborting                                // Emergency abort in progress
	PJAborted                                 // Aborted
)

func (s ProcessJobState) String() string {
	switch s {
	case PJQueued:
		return "QUEUED"
	case PJSettingUp:
		return "SETTING_UP"
	case PJWaitingForStart:
		return "WAITING_FOR_START"
	case PJProcessing:
		return "PROCESSING"
	case PJProcessComplete:
		return "PROCESS_COMPLETE"
	case PJStopping:
		return "STOPPING"
	case PJStopped:
		return "STOPPED"
	case PJAborting:
		return "ABORTING"
	case PJAborted:
		return "ABORTED"
	default:
		return "UNKNOWN"
	}
}

// ProcessJob represents a process job per SEMI E40.
type ProcessJob struct {
	JobID       string
	State       ProcessJobState
	RecipeID    string            // Process recipe
	CarrierID   string            // Source carrier
	Slots       []int             // Slot numbers to process
	Priority    int               // Job priority (higher = more urgent)
	Params      map[string]string // Process parameters
	ControlJob  string            // Parent control job ID (E94)
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
}

// ProcessJobManager manages process jobs per SEMI E40.
type ProcessJobManager struct {
	mu       sync.RWMutex
	jobs     map[string]*ProcessJob
	onChange func(jobID string, oldState, newState ProcessJobState)
}

// NewProcessJobManager creates an E40 process job manager.
func NewProcessJobManager() *ProcessJobManager {
	return &ProcessJobManager{
		jobs: make(map[string]*ProcessJob),
	}
}

// OnStateChange sets a callback for process job state transitions.
func (pm *ProcessJobManager) OnStateChange(fn func(jobID string, oldState, newState ProcessJobState)) {
	pm.mu.Lock()
	pm.onChange = fn
	pm.mu.Unlock()
}

// Create adds a new process job in Queued state.
func (pm *ProcessJobManager) Create(jobID, recipeID, carrierID string, slots []int, params map[string]string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.jobs[jobID]; ok {
		return fmt.Errorf("e40: job %s already exists", jobID)
	}

	pm.jobs[jobID] = &ProcessJob{
		JobID:     jobID,
		State:     PJQueued,
		RecipeID:  recipeID,
		CarrierID: carrierID,
		Slots:     slots,
		Params:    params,
		CreatedAt: time.Now(),
	}
	return nil
}

func (pm *ProcessJobManager) transition(jobID string, expected []ProcessJobState, newState ProcessJobState) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	job, ok := pm.jobs[jobID]
	if !ok {
		return fmt.Errorf("e40: unknown job %s", jobID)
	}

	valid := false
	for _, s := range expected {
		if job.State == s {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("e40: job %s cannot transition from %s to %s", jobID, job.State, newState)
	}

	oldState := job.State
	job.State = newState

	switch newState {
	case PJProcessing:
		job.StartedAt = time.Now()
	case PJProcessComplete, PJStopped, PJAborted:
		job.CompletedAt = time.Now()
	}

	if pm.onChange != nil {
		pm.onChange(jobID, oldState, newState)
	}
	return nil
}

// Setup transitions from Queued to SettingUp.
func (pm *ProcessJobManager) Setup(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJQueued}, PJSettingUp)
}

// SetupComplete transitions from SettingUp to WaitingForStart.
func (pm *ProcessJobManager) SetupComplete(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJSettingUp}, PJWaitingForStart)
}

// Start transitions from WaitingForStart to Processing.
func (pm *ProcessJobManager) Start(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJWaitingForStart}, PJProcessing)
}

// Complete transitions from Processing to ProcessComplete.
func (pm *ProcessJobManager) Complete(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJProcessing}, PJProcessComplete)
}

// Stop initiates a graceful stop from Processing.
func (pm *ProcessJobManager) Stop(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJProcessing}, PJStopping)
}

// StopDone transitions from Stopping to Stopped.
func (pm *ProcessJobManager) StopDone(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJStopping}, PJStopped)
}

// Abort initiates emergency abort from most states.
func (pm *ProcessJobManager) Abort(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{
		PJQueued, PJSettingUp, PJWaitingForStart, PJProcessing, PJStopping,
	}, PJAborting)
}

// AbortDone transitions from Aborting to Aborted.
func (pm *ProcessJobManager) AbortDone(jobID string) error {
	return pm.transition(jobID, []ProcessJobState{PJAborting}, PJAborted)
}

// Remove deletes a completed/stopped/aborted job.
func (pm *ProcessJobManager) Remove(jobID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	job, ok := pm.jobs[jobID]
	if !ok {
		return fmt.Errorf("e40: unknown job %s", jobID)
	}
	if job.State != PJProcessComplete && job.State != PJStopped && job.State != PJAborted {
		return fmt.Errorf("e40: job %s not in terminal state (state=%s)", jobID, job.State)
	}
	delete(pm.jobs, jobID)
	return nil
}

// GetJob returns a process job by ID.
func (pm *ProcessJobManager) GetJob(jobID string) (*ProcessJob, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	j, ok := pm.jobs[jobID]
	return j, ok
}

// ListJobs returns all process jobs.
func (pm *ProcessJobManager) ListJobs() []*ProcessJob {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*ProcessJob, 0, len(pm.jobs))
	for _, j := range pm.jobs {
		result = append(result, j)
	}
	return result
}

// ListActiveJobs returns jobs that are not in a terminal state.
func (pm *ProcessJobManager) ListActiveJobs() []*ProcessJob {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*ProcessJob
	for _, j := range pm.jobs {
		if j.State != PJProcessComplete && j.State != PJStopped && j.State != PJAborted {
			result = append(result, j)
		}
	}
	return result
}
