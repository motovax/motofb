package internal

// TimeoutError indicates an operation timed out.
type TimeoutError struct {
	Msg string
}

func (e *TimeoutError) Error() string { return e.Msg }

func ErrTimeout(msg string) error {
	return &TimeoutError{Msg: msg}
}