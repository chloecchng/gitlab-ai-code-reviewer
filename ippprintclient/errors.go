package ippprintclient

import (
	"fmt"

	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
)

type ippJobOpError struct {
	error
	retry bool
}

func (e *ippJobOpError) Temporary() bool {
	// some errors generated from the OS are considered retryable for
	// ipp since the specification says a compatible client should be
	// prepared for the printer to close the connection prematurely.
	// see: https://tools.ietf.org/html/rfc2911#section-3.1.9
	if isTemporarySyscallErr(e.error) {
		return true
	}

	// net and net/http returns possibly temporary errors
	// this is checking for those
	if netErr, ok := e.error.(interface{ Temporary() bool }); ok {
		return netErr.Temporary()
	}

	if e.retry {
		return true
	}

	return false
}

func (e *ippJobOpError) As(target any) bool {
	if t, ok := target.(*ippJobOpError); ok {
		*t = *e
		return true
	}

	return false
}

type ippStatus int16

func (s ippStatus) Recoverable() bool {
	return ippclient.Status(s) < ippclient.StatusErrorBadRequest || ippclient.Status(s) >= ippclient.StatusErrorInternal
}

// Operations specific error exit codes.
// These exist codes are returned back to the caller, and are used to determine various failure scenarios.
const (
	// Print operation specific errors.
	ErrPrintDefaultError                        int = 10 // Default error for Print operation
	ErrPrintDocFormatMismatch                   int = 11 // Document format given is not supported by printer
	ErrPrintPrinterReadyTimeout                 int = 12
	ErrPrintJobCtxTimeout                       int = 14 // Job did not complete within the timeout period (context timeout)
	ErrPrintJobCreation                         int = 15
	ErrPrintJobSendDocument                     int = 16
	ErrPrintIPPPrintJob                         int = 17 // IPP Print-Job failed
	ErrPrintJobCancelled                        int = 18
	ErrPrintJobAborted                          int = 19
	ErrPrintMonitorFailedToMonitor              int = 20 // Failed to monitor job with default IPP credentials
	ErrPrintMonitorTerminatedBeforeJobFinalised int = 21

	// Check printer operation specific errors.
	ErrCheckPrinter                 int = 30 // Default error for CheckPrinter operation
	ErrCheckPrinterPrinterNotReady  int = 31 // Printer responded properly, but printer is not ready to accept jobs
	ErrCheckPrinterErrorResponse    int = 32 // Printer responded with an error response
	ErrCheckPrinterNetwork          int = 33 // Failed to reach printer. Network error.
	ErrCheckPrinterDeviceIdMismatch int = 34 // Printer attributes printer-device-id don't match criteria
)

// OperationError : Error type to be used in operations failure.
// main() will map a particular type to an exit code.
type OperationError struct {
	Type int
	Err  error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("{ Type: %d, Args: %+v }", e.Type, e.Err)
}
