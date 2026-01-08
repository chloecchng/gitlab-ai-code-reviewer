package ippprintclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"bitbucket.org/papercutsoftware/pmitc-coordinator/util/pcerrors"

	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
)

var (
	printerAttrsToMonitor = []ippclient.AttributeName{ippclient.PrinterState, ippclient.PrinterStateReasons, ippclient.PrinterStateMessage}
)

type monitor struct {
	sync.Mutex
	ippClient    *ippclient.IPPClient
	ctx          context.Context
	printerURI   string
	jobID        int
	jobFinalised chan struct{}
	terminated   chan struct{}
	ippCreds     *ippclient.IPPCredentials
	delay        fibonnacciDelay
	attempt      int
	errs         []error

	jobState
}

type fibonnacciDelay struct {
	lastValues []time.Duration
	maxDelay   time.Duration
}

func (fd *fibonnacciDelay) nextDelay() time.Duration {
	if len(fd.lastValues) < 2 {
		fd.lastValues = append(fd.lastValues, 1*time.Second)
		return 1 * time.Second
	}

	d := fd.lastValues[0] + fd.lastValues[1]

	if d > fd.maxDelay {
		d = fd.maxDelay
		return d
	}

	fd.lastValues[0], fd.lastValues[1] = fd.lastValues[1], d

	return d
}

type jobState struct {
	*ippclient.JobAttributes
	lastCollected time.Time
}

func (m *monitor) start(ctx context.Context) {
	m.jobFinalised = make(chan struct{})
	m.terminated = make(chan struct{})
	m.ctx = ctx
	m.delay = fibonnacciDelay{maxDelay: 5 * time.Second}

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.terminated:
				return
			case <-time.After(m.delay.nextDelay()):
			}

			m.run()
		}
	}()
}

func (m *monitor) run() {
	m.Lock()
	defer m.Unlock()

	m.checkPrinterStatus()

	if m.jobID != 0 {
		m.checkJobStatus(m.jobID)
	}
}

func (m *monitor) checkPrinterStatus() {
	resp, err := m.ippClient.GetPrinterAttributes(m.printerURI, printerAttrsToMonitor)
	if err != nil {
		pclog.Devf("failed to monitor printer, err=%v", err)
		return
	}

	pclog.Devf("printer state: %d, printer state reasons: %v", resp.PrinterState, resp.PrinterStateReasons)
}

func (m *monitor) checkJobStatus(jobID int) {
	m.attempt++
	startTime := time.Now()
	jres, err := m.ippClient.GetJobAttributes(m.printerURI, jobID, []ippclient.AttributeName{
		ippclient.JobState,
		ippclient.JobStateMessage,
		ippclient.JobStateReasons,
		ippclient.JobID,
	}, m.ippCreds)
	duration := time.Since(startTime).String()
	if err == ippclient.ErrJobAttributesTagNotFound {
		msg := "GetJobAttributes: job-attributes-tag not found in response, considering this as job completed"
		pclog.Supportf(msg)
		processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
		// Like in queue printing, in this case we assume that absence of job in printer means job is printed.
		close(m.jobFinalised)
		return
	}

	if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError && reqErr != nil && reqErr.StatusCode == http.StatusNotFound {
		msg := "GetJobAttributes returned http-404, considering this as job completed"
		pclog.Supportf(msg)
		processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
		// Like in queue printing, in this case we assume that absence of job in printer means job is printed.
		close(m.jobFinalised)
		return
	}

	if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError && reqErr != nil && reqErr.StatusCode == http.StatusUnauthorized {
		if m.ippCreds != nil {
			pclog.Errorf("failed to monitor job with default IPP credentials; got %v", err)
			oe := &OperationError{
				Type: ErrPrintMonitorFailedToMonitor,
				Err:  fmt.Errorf("failed to monitor job progress: %v", err),
			}
			m.errs = append(m.errs, oe)
			close(m.jobFinalised)
			return
		}

		msg := "received HTTP 401; retrying with dummy credentials"
		pclog.Supportf(msg)
		processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
		m.ippCreds = defaultIppCredentials
		return
	}

	if err != nil {
		ippErr := &ippJobOpError{error: err}
		if ippErr.Temporary() {
			msg := fmt.Sprintf("failed to monitor job with temp error, err=%v", err)
			pclog.Devf(msg)
			processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
			return
		}

		msg := fmt.Sprintf("failed to monitor job progress: %v", err)
		oe := &OperationError{
			Type: ErrPrintMonitorFailedToMonitor,
			Err:  errors.New(msg),
		}
		processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
		m.errs = append(m.errs, oe)
		close(m.jobFinalised)
		return
	}

	m.jobState = jobState{
		JobAttributes: jres.JobAttributes,
		lastCollected: time.Now(),
	}

	// Job state transitions documented in: https://datatracker.ietf.org/doc/html/rfc2911#section-4.3.7
	msg := fmt.Sprintf("job state: %v, reasons: %v", jres.JobState, jres.JobStateReasons)
	switch jres.JobState {
	case ippclient.JobStateCompleted:
		processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
		close(m.jobFinalised)
	case ippclient.JobStateCanceled:
		oe := &OperationError{
			Type: ErrPrintJobCancelled,
			Err:  fmt.Errorf("jobID %d canceled; reasons: %v", m.jobID, m.JobStateReasons),
		}

		m.errs = append(m.errs, oe)
		close(m.jobFinalised)
	case ippclient.JobStateAborted:
		oe := &OperationError{
			Type: ErrPrintJobAborted,
			Err:  fmt.Errorf("jobID %d aborted; reasons: %v", m.jobID, m.JobStateReasons),
		}
		m.errs = append(m.errs, oe)
		close(m.jobFinalised)
	}
	processingLogger.LogOperationAttempt(getJobAttrsOperation, m.attempt, msg, duration)
}

func (m *monitor) wait() error {
	select {
	case <-m.jobFinalised:
		close(m.terminated)
		return pcerrors.Join(m.errs...)
	case <-m.terminated:
		oe := &OperationError{
			Type: ErrPrintMonitorTerminatedBeforeJobFinalised,
			Err:  fmt.Errorf("monitor terminated before job finalised"),
		}
		return oe
	case <-m.ctx.Done():
		if m.ctx.Err() != nil {
			if m.ctx.Err() == context.DeadlineExceeded {
				return &OperationError{
					Type: ErrPrintJobCtxTimeout,
					Err:  fmt.Errorf("jobID %d monitor aborted context deadline exceeded, err: %v", m.jobID, m.ctx.Err()),
				}
			} else {
				return &OperationError{
					Type: ErrPrintDefaultError,
					Err:  fmt.Errorf("jobID %d monitor aborted err: %v", m.jobID, m.ctx.Err()),
				}
			}
		}
		return nil
	}
}

func (m *monitor) setJobID(jobID int) {
	m.Lock()
	defer m.Unlock()

	m.jobID = jobID
}

func (m *monitor) unsetJobID() {
	m.Lock()
	defer m.Unlock()
	// 0 is an invalid IPP job ID - we use that as the "unset" value here
	m.jobID = 0
}
