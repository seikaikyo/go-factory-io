package secs2

import "errors"

var (
	ErrInvalidBody   = errors.New("secs2: body must be *Item")
	ErrTrailingData  = errors.New("secs2: trailing data after decode")
	ErrInvalidFormat = errors.New("secs2: invalid format code")
)
