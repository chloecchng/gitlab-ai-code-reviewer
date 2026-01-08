package ippprintclient

import (
	"fmt"
	"io"
	"sync"
)

type ippclientProcessingLogger struct {
	mu     sync.Mutex // Serialise access to the output Writer.
	output io.Writer
}

// LogOperationAttempt Log the given info to the output Writer. With processingReportMarker & a timestamp prepended to it.
func (p *ippclientProcessingLogger) LogOperationAttempt(operation string, attempt int, note string, time string) {
	s := fmt.Sprintf("%s%s: attempt %d - %s, time - %s\n", processingReportMarker, operation, attempt, note, time)
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = p.output.Write([]byte(s))
}
