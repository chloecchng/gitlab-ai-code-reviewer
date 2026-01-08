package ippprintclient

import (
	"sync"

	"bitbucket.org/papercutsoftware/gopapercut/pclog"
)

var (
	processingLogger ProcessingLogger = &defaultProcessingLogger{}
	once             sync.Once
)

type defaultProcessingLogger struct {
}

type ProcessingLogger interface {
	LogOperationAttempt(operation string, attempt int, note string, duration string)
}

func (p *defaultProcessingLogger) LogOperationAttempt(operation string, attempt int, note string, duration string) {
	once.Do(processingLoggerNotSet)
}

func processingLoggerNotSet() {
	pclog.Supportf("processing logger is not set")
}
