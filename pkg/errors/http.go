package errors

import "go.rtnl.ai/x/api"

// HTTP is a custom error type that allows us to return an HTTP status code and
// a JSON reply from a function. This is useful for functions that need to indicate
// what to return in a gin handler without having to accept the gin context.
type HTTP struct {
	Status int   // status code to return to the client
	Err    error // the original error
	Reply  any   // the output of api.Error() --> using any to avoid circular imports
}

func (e *HTTP) Error() string {
	return e.Err.Error()
}

func (e *HTTP) Unwrap() error {
	return e.Err
}

// This is essentially a duplicate of api.Error and is used to construct an HTTP
// error response. This function is to prevent recursive imports but api.Error is
// preferred where possible because this function does not handle validation errors.
func Reply(status int, err error, msg string) *HTTP {
	if err == nil {
		return &HTTP{
			Status: status,
			Err:    ErrUnknown,
			Reply:  api.Reply{Success: false, Error: ErrUnknown.Error()},
		}
	}

	if msg == "" {
		msg = err.Error()
	}

	return &HTTP{
		Status: status,
		Err:    err,
		Reply:  api.Reply{Success: false, Error: msg},
	}
}
