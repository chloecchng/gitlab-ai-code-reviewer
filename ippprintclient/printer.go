package ippprintclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
)

type ippPrinter struct {
	Credentials   *ippclient.IPPCredentials
	TmpDir        string
	ippClient     *ippclient.IPPClient
	monitor       *monitor
	retryAttempts int
}

// we currently don't support multi document print operations. So this is always true
const (
	lastDocumentFlag = true
	maxPrintLoops    = 4
)

var retryBackoffSeconds int64 = 5

// IPP/1.1 RFC8011: https://tools.ietf.org/html/rfc8011
func (p *ippPrinter) CreateSendDocument(ctx context.Context, jobTemplate *ippclient.PrintJobTemplateAttributes, printerURI string, r io.ReadCloser, docFormat string) (*ippclient.JobAttributes, error) {
	tmpFile, err := os.CreateTemp(p.TmpDir, "pcippclient-*")
	if err != nil {
		return nil, &OperationError{
			Type: ErrPrintDefaultError,
			Err:  fmt.Errorf("failed to create temporary file: %v", err),
		}
	}

	// temp file cleanup
	defer func() {
		err := tmpFile.Close()
		if err != nil {
			pclog.Devf("err=%v", err)
		}

		err = os.Remove(tmpFile.Name())
		if err != nil {
			pclog.Devf("failed to remove spoolfile: %v", err)
		}
	}()

	var docReader = &streamReader{
		ReadCloser: io.NopCloser(io.TeeReader(r, tmpFile)),
		tmpFile:    tmpFile,
	}

	var job *ippclient.JobAttributes
	for p.retryAttempts = 0; p.retryAttempts < maxPrintLoops; p.retryAttempts++ {

		if p.retryAttempts > 0 {
			jitter := rand.Int63n(retryBackoffSeconds)
			<-time.After(time.Duration(retryBackoffSeconds+jitter) * time.Second)
		}

		resp, err := p.createJob(ctx, jobTemplate, printerURI)
		if err != nil {
			pclog.Errorf("failed to create job; err: %v", err)

			ippErr := &ippJobOpError{}
			if errors.As(err, ippErr) && ippErr.Temporary() {
				continue
			}

			return nil, &OperationError{
				Type: ErrPrintJobCreation,
				Err:  fmt.Errorf("ipp Create-Job failed: %v", err),
			}
		}

		p.monitor.setJobID(resp.JobId)

		_, err = p.sendDocument(ctx, printerURI, resp.JobUri, resp.JobAttributes, docReader, docFormat)
		if err != nil {
			pclog.Errorf("failed to send document; err: %v", err)

			ippErr := &ippJobOpError{}
			if errors.As(err, ippErr) && ippErr.Temporary() {
				continue
			}

			return nil, &OperationError{
				Type: ErrPrintJobSendDocument,
				Err:  fmt.Errorf("ipp Send-Document failed: %v", err),
			}
		}

		job = &ippclient.JobAttributes{
			JobId:           resp.JobId,
			JobUri:          resp.JobUri,
			JobState:        resp.JobState,
			JobStateMessage: resp.JobStateMessage,
			JobStateReasons: resp.JobStateReasons,
		}
		break
	}

	return job, nil
}

// IPP/1.0 RFC2911: https://tools.ietf.org/html/rfc2566
func (p *ippPrinter) PrintJob(ctx context.Context, jobTemplate *ippclient.PrintJobTemplateAttributes, printerURI string, r io.ReadCloser, docFormat string) (*ippclient.JobAttributes, error) {
	tmpFile, err := os.CreateTemp(p.TmpDir, "pcippclient-*")
	if err != nil {
		return nil, &OperationError{
			Type: ErrPrintDefaultError,
			Err:  fmt.Errorf("failed to create temporary file: %v", err),
		}
	}

	// temp file cleanup
	defer func() {
		err := tmpFile.Close()
		if err != nil {
			pclog.Devf("err=%v", err)
		}

		err = os.Remove(tmpFile.Name())
		if err != nil {
			pclog.Devf("failed to remove spoolfile: %v", err)
		}
	}()

	isRetry := false
	retryWithDefaultCredentials := false

	var docReader readCloseResetter
	docReader = &streamReader{
		ReadCloser: io.NopCloser(io.TeeReader(r, tmpFile)),
		tmpFile:    tmpFile,
	}

	printJobRetryAttempts := 0
	var job *ippclient.JobAttributes
	for {
		if isRetry {
			docReader, err = docReader.Reset()
			if err != nil {
				return nil, &OperationError{
					Type: ErrPrintDefaultError,
					Err:  fmt.Errorf("failed to read document: %v", err),
				}
			}
			// backoff unless retrying with default credentials
			if !retryWithDefaultCredentials {
				jitter := rand.Int63n(retryBackoffSeconds)
				<-time.After(time.Duration(retryBackoffSeconds+jitter) * time.Second)
			}
		}

		if !isRetry {
			isRetry = true
		}

		if retryWithDefaultCredentials {
			p.Credentials = defaultIppCredentials
		}

		printJobRetryAttempts = printJobRetryAttempts + 1
		if printJobRetryAttempts > *ippMaxPrintJobSendDocumentRetryAttempts {
			return nil, &OperationError{
				Type: ErrPrintIPPPrintJob,
				Err: fmt.Errorf(
					"ipp Print-Job failed, err: max operation retry attempts %d exceeded",
					*ippMaxPrintJobSendDocumentRetryAttempts,
				),
			}
		}

		startTime := time.Now()
		resp, err := p.printJob(ctx, printerURI, jobTemplate, docReader, docFormat)
		duration := time.Since(startTime).String()
		pclog.Devf("print-job responded in %v", time.Since(startTime))

		if err != nil {
			pclog.Errorf("failed to print job; err: %v", err)
			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError && reqErr != nil &&
				reqErr.StatusCode == http.StatusUnauthorized {
				if !retryWithDefaultCredentials {
					msg := "retry with default ipp credentials"
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(printJobOperation, printJobRetryAttempts, msg, duration)
					retryWithDefaultCredentials = true
					continue
				}

				// CUPS retries 4 times on HTTP 401 most probably to get around printer quirks
				// we're trying to mimic its behaviour here by retrying a set number of times.
				if printJobRetryAttempts <= *ippMaxUnauthorisedAttempts {
					msg := fmt.Sprintf("Print-Job: received HTTP 401; trying again - attempt %d/%d", printJobRetryAttempts, *ippMaxUnauthorisedAttempts)
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(printJobOperation, printJobRetryAttempts, msg, duration)
					continue
				}

				return nil, &OperationError{
					Type: ErrPrintIPPPrintJob,
					Err:  fmt.Errorf("ipp Print-Job failed: %v", err),
				}
			}

			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError {
				//just log for now
				msg := fmt.Sprintf("failed to print job, http reqErr code %v, err: %v", reqErr, err)
				pclog.Errorf(msg)
				processingLogger.LogOperationAttempt(printJobOperation, printJobRetryAttempts, msg, duration)
			}

			ippErr := &ippJobOpError{error: err}
			if ippErr.Temporary() {
				msg := fmt.Sprintf("encountered temporary network error: %v", ippErr)
				pclog.Supportf(msg)
				processingLogger.LogOperationAttempt(printJobOperation, printJobRetryAttempts, msg, duration)
				continue
			}

			return nil, &OperationError{
				Type: ErrPrintIPPPrintJob,
				Err:  fmt.Errorf("ipp Print-Job failed: %v", err),
			}
		}

		if !resp.StatusCode.IsStatusOK() {
			msg := fmt.Sprintf("Print-Job operation failed with status %s", resp.StatusMessage())
			pclog.Supportf(msg)
			processingLogger.LogOperationAttempt(printJobOperation, printJobRetryAttempts, msg, duration)
			continue
		}

		p.monitor.setJobID(resp.JobId)

		job = &ippclient.JobAttributes{
			JobId:           resp.JobId,
			JobUri:          resp.JobUri,
			JobState:        resp.JobState,
			JobStateMessage: resp.JobStateMessage,
			JobStateReasons: resp.JobStateReasons,
		}
		break
	}

	return job, nil
}

func (p *ippPrinter) createJob(ctx context.Context, jobTemplate *ippclient.PrintJobTemplateAttributes, printerURI string) (*ippclient.CreateJobResponse, error) {
	createJobAttempts := 0
	skipBackoff := false
	var resp *ippclient.CreateJobResponse
	var err error

	for {
		if err = ctx.Err(); err != nil {
			msg := fmt.Sprintf("failed: %v", err)
			processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, "")
			return nil, err
		}

		if createJobAttempts > 0 && !skipBackoff {
			jitter := rand.Int63n(retryBackoffSeconds)
			<-time.After(time.Duration(retryBackoffSeconds+jitter) * time.Second)
		} else {
			skipBackoff = false
		}

		createJobAttempts++

		createJobStartTime := time.Now()
		resp, err = p.ippClient.CreateJob(printerURI, jobTemplate, p.Credentials)
		createJobDuration := time.Since(createJobStartTime).String()

		if err != nil {
			pclog.Errorf("failed to create the job; err: %v", err)
			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError && reqErr != nil && reqErr.StatusCode == http.StatusUnauthorized {
				if p.Credentials == nil {
					msg := "retry with default ipp credentials"
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)
					p.Credentials = defaultIppCredentials
					skipBackoff = true
					continue
				}

				// CUPS retries 4 times on HTTP 401 most probably to get around printer quirks
				// we're trying to mimic its behaviour here by retrying a set number of times.
				if createJobAttempts <= *ippMaxUnauthorisedAttempts {
					msg := fmt.Sprintf("Create-Job received HTTP 401; trying again - attempt %d/%d", createJobAttempts, *ippMaxUnauthorisedAttempts)
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)
					continue
				}

				processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, err.Error(), createJobDuration)
				return nil, fmt.Errorf("failed to create job: %v", err)
			}

			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError {
				//just log for now
				msg := fmt.Sprintf("failed to create job, err: http reqErr code %v", reqErr)
				pclog.Errorf(msg)
				processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)
			}

			ippErr := &ippJobOpError{error: err}
			if ippErr.Temporary() {
				msg := fmt.Sprintf("encountered temporary network error: %v", ippErr)
				pclog.Supportf(msg)
				processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)
				continue
			}

			processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, err.Error(), createJobDuration)
			return nil, fmt.Errorf("CreateJob failed with unrecoverable error: %v", err)
		}

		if !resp.StatusCode.IsStatusOK() {
			msg := fmt.Sprintf("create job request failed with status %s", resp.StatusMessage())
			pclog.Supportf(msg)
			processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)

			if ippStatus(resp.StatusCode).Recoverable() {
				pclog.Devf("received recoverable IPP status %s", resp.StatusMessage())
				continue
			}

			return resp, fmt.Errorf(msg)
		}

		// validate the job-id returned by the printer.
		// job-id must be within the range: integer(1:MAX), where MAX = 2**31 - 1.
		// https://datatracker.ietf.org/doc/html/rfc8011#section-5.3.
		if resp.JobId <= 0 || resp.JobId > math.MaxInt32 {
			// we try to recreate the job upto a certain number of attempts.
			// If we keep getting invalid job IDs, fail the job
			if createJobAttempts < *maxCreateJobAttempts {
				pclog.Supportf("failed to validate job-id(%v); retrying...", resp.JobId)
				continue
			}
			pclog.Supportf("failed to validate job-id(%v); retries exhausted...exit!", resp.JobId)
			return nil, fmt.Errorf("failed to create job:invalid job-id %v", resp.JobId)
		}

		msg := fmt.Sprintf("create-job response status code: %v, jobId: %v", resp.StatusCode, resp.JobId)
		processingLogger.LogOperationAttempt(createJobOperation, createJobAttempts, msg, createJobDuration)
		break
	}

	return resp, nil
}

func (p *ippPrinter) sendDocument(ctx context.Context, printerURI, jobURI string, jobAttributes *ippclient.JobAttributes, file readCloseResetter, docFormat string) (*ippclient.SendDocumentResponse, error) {
	sendDocAttempts := 0
	skipBackoff := false
	var sendDocResp *ippclient.SendDocumentResponse
	var err error

	for {
		sendDocumentStartTime := time.Now()

		if err = ctx.Err(); err != nil {
			return nil, err
		}

		if sendDocAttempts > 0 {
			file, err = file.Reset()
			if err != nil {
				processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, err.Error(), time.Since(sendDocumentStartTime).String())
				return nil, fmt.Errorf("failed to read document: %v", err)
			}

			if !skipBackoff {
				jitter := rand.Int63n(retryBackoffSeconds)
				<-time.After(time.Duration(retryBackoffSeconds+jitter) * time.Second)
			} else {
				skipBackoff = false
			}
		}

		sendDocAttempts++

		if sendDocAttempts > *ippMaxPrintJobSendDocumentRetryAttempts {
			msg := fmt.Sprintf("failed to send document, err: max operation retry attempts %d exceeded", *ippMaxPrintJobSendDocumentRetryAttempts)
			processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, time.Since(sendDocumentStartTime).String())
			return nil, fmt.Errorf(msg)
		}

		sendDocResp, err = p.ippClient.SendDocument(printerURI, jobURI, &ippclient.Document{
			Format: docFormat,
			Reader: file,
		}, lastDocumentFlag, p.Credentials)
		ippInfo := fromSendDocumentResponse(sendDocResp)

		sendDocumentDuration := time.Since(sendDocumentStartTime).String()
		pclog.Supportf("send-document responded in %v", time.Since(sendDocumentStartTime))

		if err != nil {
			pclog.Errorf("failed to send document for job %d", jobAttributes.JobId)
			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError &&
				reqErr != nil &&
				reqErr.StatusCode == http.StatusUnauthorized {
				if p.Credentials == nil {
					msg := "retry with default ipp credentials"
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
					p.Credentials = defaultIppCredentials
					skipBackoff = true
					continue
				}

				// CUPS retries 4 times on HTTP 401 most probably to get around printer quirks
				// we're trying to mimic its behaviour here by retrying a set number of times.
				if sendDocAttempts <= *ippMaxUnauthorisedAttempts {
					msg := fmt.Sprintf("Send-Document received HTTP 401; trying again - attempt %d/%d", sendDocAttempts, *ippMaxUnauthorisedAttempts)
					pclog.Supportf(msg)
					processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
					continue
				}

				p.cancelJob(printerURI, jobAttributes.JobId)

				msg := fmt.Sprintf("Send-Document failed with ipp response: %+v", ippInfo)
				processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
				return nil, fmt.Errorf("failed to send document: %v", err)
			}

			if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError {
				//just log for now
				msg := fmt.Sprintf("failed to send document, http reqErr code %v, err:%v", reqErr, err)
				pclog.Errorf(msg)
				processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
			}

			ippErr := &ippJobOpError{error: err}
			if ippErr.Temporary() {
				msg := fmt.Sprintf("encountered temporary network error: %v", ippErr)
				pclog.Supportf(msg)
				processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
				p.cancelJob(printerURI, jobAttributes.JobId)
				continue
			}

			msg := fmt.Sprintf("failed to send document with ippResponse: %+v", ippInfo)
			processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
			p.cancelJob(printerURI, jobAttributes.JobId)
			return nil, fmt.Errorf("failed to send document: %v", err)
		}

		if !sendDocResp.StatusCode.IsStatusOK() {
			p.cancelJob(printerURI, jobAttributes.JobId)
			msg := fmt.Sprintf("Send-Document operation failed with status %s, ippStatus %+v", sendDocResp.StatusMessage(), ippInfo)
			pclog.Supportf(msg)
			processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)

			if ippStatus(sendDocResp.StatusCode).Recoverable() {
				pclog.Devf("received recoverable status %s", sendDocResp.StatusMessage())
				continue
			}

			return nil, fmt.Errorf(msg)
		}

		msg := fmt.Sprintf("send-document response status code: %v, ippStatus: %+v", sendDocResp.StatusCode, ippInfo)
		processingLogger.LogOperationAttempt(sendDocumentOperation, sendDocAttempts, msg, sendDocumentDuration)
		break
	}

	return sendDocResp, nil
}

func (p *ippPrinter) cancelJob(printerURI string, jobID int) {
	pclog.Devf("attempting to cancel job %d", jobID)
	startTime := time.Now()

	// unset the jobID on the monitor first. This will make the job monitor
	// wait until a new jobID is available to monitor it again.
	p.monitor.unsetJobID()

	resp, err := p.ippClient.CancelJob(printerURI, jobID, p.Credentials)
	if err != nil {
		pclog.Devf("failed to cancel job: %v", err)
		return
	}

	if !resp.StatusCode.IsStatusOK() {
		pclog.Devf("failed to cancel job: %v", err)
		return
	}

	duration := time.Since(startTime).String()
	msg := fmt.Sprintf("response status code - %v", resp.StatusCode)
	processingLogger.LogOperationAttempt(cancelJobOperation, 1, msg, duration)

	pclog.Devf("job %d cancelled", jobID)
}

func (p *ippPrinter) printJob(ctx context.Context, printerURI string, jobTemplate *ippclient.PrintJobTemplateAttributes, file io.ReadCloser, docFormat string) (*ippclient.PrintJobResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	printJobResp, err := p.ippClient.PrintJob(printerURI, &ippclient.Document{
		Format: docFormat,
		Reader: file,
	}, jobTemplate, p.Credentials)

	ippInfo := fromPrintJobResponse(printJobResp)
	if err != nil {
		msg := fmt.Sprintf("Print-Job operation failed with err: %v, ippStatus: %+v }", err.Error(), ippInfo)
		processingLogger.LogOperationAttempt(printJobOperation, 1, msg, "")
		return nil, err
	}

	return printJobResp, nil
}

type ippLogInfo struct {
	JobState        int
	JobStateMessage string
	JobStateReasons []string
}

func fromPrintJobResponse(resp *ippclient.PrintJobResponse) *ippLogInfo {
	if resp == nil {
		return nil
	}

	return &ippLogInfo{
		JobState:        resp.JobState,
		JobStateMessage: resp.JobStateMessage,
		JobStateReasons: resp.JobStateReasons,
	}
}

func fromSendDocumentResponse(resp *ippclient.SendDocumentResponse) *ippLogInfo {
	if resp == nil {
		return nil
	}

	return &ippLogInfo{
		JobState:        resp.JobState,
		JobStateMessage: resp.JobStateMessage,
		JobStateReasons: resp.JobStateReasons,
	}
}
