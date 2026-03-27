package gem

import (
	"context"
	"fmt"
	"sync"
)

// CommandStatus is the result of a remote command execution.
type CommandStatus byte

const (
	CommandOK                CommandStatus = 0 // HCACK: command performed
	CommandInvalidCommand    CommandStatus = 1 // HCACK: command does not exist
	CommandCannotPerform     CommandStatus = 2 // HCACK: cannot perform now
	CommandParameterError    CommandStatus = 3 // HCACK: at least one parameter invalid
	CommandAckFinishLater    CommandStatus = 4 // HCACK: acknowledged, will finish later
	CommandRejected          CommandStatus = 5 // HCACK: rejected, already in desired condition
)

func (s CommandStatus) String() string {
	switch s {
	case CommandOK:
		return "OK"
	case CommandInvalidCommand:
		return "INVALID_COMMAND"
	case CommandCannotPerform:
		return "CANNOT_PERFORM"
	case CommandParameterError:
		return "PARAMETER_ERROR"
	case CommandAckFinishLater:
		return "ACK_FINISH_LATER"
	case CommandRejected:
		return "REJECTED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// CommandParam is a name-value pair for remote command parameters.
type CommandParam struct {
	Name  string
	Value interface{}
}

// CommandHandlerFunc processes a remote command and returns a status.
type CommandHandlerFunc func(ctx context.Context, params []CommandParam) CommandStatus

// CommandManager manages remote commands (RCMD) per SEMI E30.
type CommandManager struct {
	mu       sync.RWMutex
	commands map[string]CommandHandlerFunc
}

// NewCommandManager creates an empty command manager.
func NewCommandManager() *CommandManager {
	return &CommandManager{
		commands: make(map[string]CommandHandlerFunc),
	}
}

// Register registers a remote command handler.
func (cm *CommandManager) Register(name string, handler CommandHandlerFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.commands[name] = handler
}

// Execute runs a remote command. Returns CommandInvalidCommand if not found.
func (cm *CommandManager) Execute(ctx context.Context, name string, params []CommandParam) CommandStatus {
	cm.mu.RLock()
	handler, ok := cm.commands[name]
	cm.mu.RUnlock()

	if !ok {
		return CommandInvalidCommand
	}
	return handler(ctx, params)
}

// ListCommands returns all registered command names.
func (cm *CommandManager) ListCommands() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	names := make([]string, 0, len(cm.commands))
	for name := range cm.commands {
		names = append(names, name)
	}
	return names
}

// HasCommand checks if a command is registered.
func (cm *CommandManager) HasCommand(name string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.commands[name]
	return ok
}
