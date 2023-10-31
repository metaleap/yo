package jobs

type Logger interface {
	Background() Logger
	WithError(...any) Logger
	Errorf(...any)
	Infof(...any)
}

func loggerFor(...any) Logger {
	panic("TODO")
}

func isErrRetryable(...any) bool {
	panic("TODO")
}

func newULID() (string, error) {
	panic("TODO")
}

func WithTimeoutDo(...any) {
	panic("TODO")
}

func zapString(...any) any {
	panic("TODO")
}

func logWith(...any) Logger {
	panic("TODO")
}
