// Package modbus implements a Modbus TCP client for PLC and sensor communication.
// Pure Go implementation with no external dependencies.
package modbus

import "fmt"

// ExceptionCode is a Modbus exception response code.
type ExceptionCode byte

const (
	ExcIllegalFunction     ExceptionCode = 0x01
	ExcIllegalDataAddress  ExceptionCode = 0x02
	ExcIllegalDataValue    ExceptionCode = 0x03
	ExcSlaveDeviceFailure  ExceptionCode = 0x04
	ExcAcknowledge         ExceptionCode = 0x05
	ExcSlaveDeviceBusy     ExceptionCode = 0x06
	ExcMemoryParityError   ExceptionCode = 0x08
	ExcGatewayPathUnavail  ExceptionCode = 0x0A
	ExcGatewayTargetFailed ExceptionCode = 0x0B
)

func (e ExceptionCode) String() string {
	switch e {
	case ExcIllegalFunction:
		return "Illegal Function"
	case ExcIllegalDataAddress:
		return "Illegal Data Address"
	case ExcIllegalDataValue:
		return "Illegal Data Value"
	case ExcSlaveDeviceFailure:
		return "Slave Device Failure"
	case ExcAcknowledge:
		return "Acknowledge"
	case ExcSlaveDeviceBusy:
		return "Slave Device Busy"
	case ExcMemoryParityError:
		return "Memory Parity Error"
	case ExcGatewayPathUnavail:
		return "Gateway Path Unavailable"
	case ExcGatewayTargetFailed:
		return "Gateway Target Device Failed to Respond"
	default:
		return fmt.Sprintf("Unknown Exception (0x%02X)", byte(e))
	}
}

// ModbusError represents a Modbus exception response from a slave device.
type ModbusError struct {
	FunctionCode  byte
	ExceptionCode ExceptionCode
}

func (e *ModbusError) Error() string {
	return fmt.Sprintf("modbus: function 0x%02X exception: %s", e.FunctionCode, e.ExceptionCode)
}
