package fsutil

import (
	"fmt"
	"sync"
)

// Avoid importing the log type information unnecessarily.  There's a small cost
// associated with using an interface rather than the type.  Depending on how
// often the logger is plugged in, it would be worth using the type instead.
type log_Logger interface {
	Output(calldepth int, s string) error
}

var globalLoggerLock sync.Mutex
var globalLogger log_Logger
var globalDebug bool

// Specify the *log.Logger object where log messages should be sent to.
func SetLogger(logger log_Logger) {
	globalLoggerLock.Lock()
	globalLogger = logger
	globalLoggerLock.Unlock()
}

// Enable the delivery of debug messages to the logger.  Only meaningful
// if a logger is also set.
func SetDebug(debug bool) {
	globalLoggerLock.Lock()
	globalDebug = debug
	globalLoggerLock.Unlock()
}

// logf sends to the logger registered via SetLogger the string resulting
// from running format and args through Sprintf.
func logf(format string, args ...interface{}) {
	globalLoggerLock.Lock()
	defer globalLoggerLock.Unlock()
	if globalLogger != nil {
		globalLogger.Output(2, fmt.Sprintf(format, args...))
	}
}

// debugf sends to the logger registered via SetLogger the string resulting
// from running format and args through Sprintf, but only if debugging was
// enabled via SetDebug.
func debugf(format string, args ...interface{}) {
	globalLoggerLock.Lock()
	defer globalLoggerLock.Unlock()
	if globalDebug && globalLogger != nil {
		globalLogger.Output(2, fmt.Sprintf(format, args...))
	}
}
