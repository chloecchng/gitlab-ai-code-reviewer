package ippprintclient

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"bitbucket.org/papercutsoftware/pmitc-coordinator/util/info"

	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient/printerattributecache"

	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/gopapercut/print/ipp"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3/finishings"
	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient/jobticket"
	"bitbucket.org/papercutsoftware/pmitc-coordinator/util/config"
)

var defaultIppCredentials = &ippclient.IPPCredentials{Username: "papercut-ipp-client", Password: "papercut"}
var preferredJobOperations = []ipp.Operation{ipp.OperationCreateJob, ipp.OperationSendDocument}

const (
	createJobOperation       = "create-job"
	sendDocumentOperation    = "send-document"
	printJobOperation        = "print-job"
	cancelJobOperation       = "cancel-job"
	validateJobOperation     = "validate-job"
	getPrinterAttrsOperation = "get-printer-attributes"
	getJobAttrsOperation     = "get-job-attributes"
)

func printJob(ticketPath, printerURI string,
	file io.ReadCloser,
	httpClient ippclient.HttpClientInterface,
	attribCache *printerattributecache.PrinterAttributeCache,
	printTimeout time.Duration) error {

	opErr := &OperationError{
		Type: ErrPrintDefaultError,
	}

	if ticketPath == "" || printerURI == "" {
		flag.PrintDefaults()
		opErr.Err = fmt.Errorf("ticketpath or printerURI empty")
		return opErr
	}

	ticketAttrs, err := jobticket.GetPrintJobAttributes(ticketPath)
	if err != nil {
		pclog.Errorf("failed to map job attributes from print action. %v", err)
		opErr.Err = fmt.Errorf("failed to read ticket")
		return opErr
	}

	var ippCreds *ippclient.IPPCredentials = nil
	if ticketAttrs.Credentials.Username != "" && ticketAttrs.Credentials.Password != "" {
		ippCreds = &ippclient.IPPCredentials{
			Username: ticketAttrs.Credentials.Username,
			Password: ticketAttrs.Credentials.Password,
		}
	}
	ippClient, err := ippclient.NewIPPClient(ippclient.SetHTTPClient(httpClient))
	if err != nil {
		opErr.Err = fmt.Errorf("failed to create ipp ippClient, err: %v", err)
		return opErr
	}
	defer func() {
		err := ippClient.Close()
		if err != nil {
			pclog.Devf("failed to close ipp client: %v", err)
		}
	}()

	pclog.Devf("requesting PrintJob with printerURI: %v, ticketAttrs: %+v", printerURI, ticketAttrs)
	// Context with deadline for the whole printing operation.
	// Sanity check here to prevent this going below 1 hour purely because we had a hardcoded one hour for
	// sometime in the field. We don't want to regress things by making it shorter.
	if printTimeout < time.Hour {
		printTimeout = time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), printTimeout)
	defer cancel()

	var printerAttributes *ippclient.PrinterAttributes = nil

	// Try to get the ipp-printer-attributes from cache.
	// This is a best effort only, if the printer data is not cached it's not an error.

	if attribCache != nil {
		printerAttributes, err = attribCache.GetPrinterAttributes(printerURI)
		if err == nil {
			processingLogger.LogOperationAttempt(printJobOperation, 1, "ipp-printer-attribute-cache: Found", "0")
			pclog.Devf("ipp-printer-attribute-cache: Found printer attributes for: %v", printerURI)
		}
	} else {
		processingLogger.LogOperationAttempt(printJobOperation, 1, "ipp-printer-attribute-cache: Not Found", "0")
		pclog.Supportf("ipp-printer-attribute-cache: Failed to get cached attributes: %v - %v,"+
			" reaching the printer", printerURI, err)
	}

	if err != nil || printerAttributes == nil {
		// If failure to get from cache, or the printer info doesn't exist in cache, reach the printer.
		printerAttributes, err = waitForPrinterReady(ctx, printerURI, ippClient, ippCreds)
		if err != nil {
			pclog.Errorf("waitForPrinterReady Failed: %v - %v", printerURI, err)
			return err
		} else {
			if attribCache != nil {
				// Try to set it in cache. If it fails, don't fail the print job.
				_ = attribCache.SetPrinterAttributes(printerURI, printerAttributes)
			}
		}
	}

	jobTemplateAttrs := makeIPPJobAttributes(ticketAttrs, printerAttributes)
	pclog.Supportf("got ipp attrs for job: %v", jobTemplateAttrs)

	selectedDocFormat := mapDocumentFormat(ticketAttrs, printerAttributes)
	if selectedDocFormat == "" {
		pclog.Errorf("document format not supported :printing=%s|supported=%v failed",
			ticketAttrs.DocumentFormat, printerAttributes.DocumentFormatSupported)
		return &OperationError{
			Type: ErrPrintDocFormatMismatch,
			Err: fmt.Errorf("document format not supported :printing=%s|supported=%v failed",
				ticketAttrs.DocumentFormat, printerAttributes.DocumentFormatSupported),
		}
	}

	monitor := &monitor{
		printerURI: printerURI,
		ippClient:  ippClient,
		ippCreds:   ippCreds,
	}
	monitor.start(ctx)

	printer := &ippPrinter{
		ippClient:   ippClient,
		Credentials: ippCreds,
		TmpDir:      config.TmpDir,
		monitor:     monitor,
	}

	errChan := make(chan error)
	monitorCompleteChan := make(chan struct{})

	go func(ctx context.Context) {
		if !(*ippPrintOperation == "\"print-job\"" || *ippPrintOperation == "print-job") && operationsSupported(printerAttributes, preferredJobOperations) {
			pclog.Devf("Printing job using CreateSendDocument operation, document-format=%v", selectedDocFormat)
			_, err = printer.CreateSendDocument(ctx, jobTemplateAttrs, printerURI, file, selectedDocFormat)
		} else {
			pclog.Devf("Printing job using Print-Job operation, document-format=%v", selectedDocFormat)
			_, err = printer.PrintJob(ctx, jobTemplateAttrs, printerURI, file, selectedDocFormat)
		}

		if err != nil {
			pclog.Errorf("failed to print job: %v", err)
			var oe *OperationError
			if errors.As(err, &oe) {
				errChan <- err
				return
			}
			opErr.Err = err
			errChan <- opErr
		}
	}(ctx)

	go func(ctx context.Context) {
		err = monitor.wait()
		if err != nil {
			pclog.Errorf("job did not complete successfully: %v", err)
			// Log it as this is visible from the cloud/BQ. This is job-monitoring failure, so still using getJobAttrsOperation.
			var oe *OperationError
			if errors.As(err, &oe) {
				errChan <- oe
				return
			}
			// The underlying layer is not sending back a categorised OperationError,
			// send back the default print error.
			errChan <- &OperationError{
				Type: ErrPrintDefaultError,
				Err:  fmt.Errorf("job did not complete successfully: %v", err),
			}
			return
		}

		// monitoring will complete once the job is finalised
		close(monitorCompleteChan)
	}(ctx)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err = <-errChan:
		// cancel context when an error occurs
		cancel()
		return err
	case <-monitorCompleteChan:
		cancel()
		return nil
	}
}

// waitForPrinterReady Wait for printer to be ready. Poll the printer for IPP attributes,
// and wait till it's ready with timeout. Default printer ready timeout is 600sec.
// Returns an OperationError at failure.
func waitForPrinterReady(
	ctx context.Context,
	printerURI string,
	ippClient *ippclient.IPPClient,
	ippCreds *ippclient.IPPCredentials,
) (*ippclient.PrinterAttributes, error) {

	printerReadyTimeout := time.NewTimer(time.Duration(*printerReadyTimeoutSec) * time.Second)
	defer printerReadyTimeout.Stop()

	var printerAttributes *ippclient.PrinterAttributes
	attempts := 0
getPrinterAttrs:
	for {
		select {
		case <-printerReadyTimeout.C:
			pclog.Supportf("print request timed out: waited %d seconds", *printerReadyTimeoutSec)
			return nil, &OperationError{
				Type: ErrPrintPrinterReadyTimeout,
				Err:  fmt.Errorf("printer ready timeout: waited %v seconds", *printerReadyTimeoutSec),
			}
		case <-ctx.Done():
			pclog.Supportf("print operation terminated: context timeout while waiting for printer ready")
			if ctx.Err() == context.DeadlineExceeded {
				return nil, &OperationError{
					Type: ErrPrintJobCtxTimeout,
					Err:  fmt.Errorf("context timeout while waiting for printer ready"),
				}
			} else {
				return nil, &OperationError{
					Type: ErrPrintDefaultError,
					Err:  fmt.Errorf("context done/error while waiting for printer ready: %v", ctx.Err()),
				}
			}
		default:
			pclog.Devf("getting printer attributes over ipp")
			attempts++
			//TODO: future: do get printer attribute in a separate thread
			getPrinterAttrsOpStartTime := time.Now()
			printerAttrsResponse, err := ippClient.GetPrinterAttributes(printerURI, printerReadyAttributes, ippCreds)
			duration := time.Since(getPrinterAttrsOpStartTime).String()
			if err != nil && err != ippclient.ErrMalformedAttributes {
				if reqErr, isHttpStatusError := ippclient.IsHTTPStatusError(err); isHttpStatusError {
					// TODO: Check here - do we exit with error.
					msg := fmt.Sprintf("failed to get printer attributes, err: http reqErr code %v", reqErr)
					pclog.Errorf(msg)
					processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempts, msg, duration)
				} else {
					msg := fmt.Sprintf("failed to get printer attributes, err: %v, retry in %d sec",
						err, *printerReadyDelaySec)
					processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempts, msg, duration)
				}
				time.Sleep(time.Duration(*printerReadyDelaySec) * time.Second)
				continue
			}

			if ready, reason := isPrinterReady(printerAttrsResponse.PrinterAttributes); !ready {
				msg := fmt.Sprintf("printer is not ready to accept job: printer state reason: %v, retry in %d sec",
					reason, *printerReadyDelaySec)
				pclog.Errorf(msg)
				processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempts, msg, duration)
				time.Sleep(time.Duration(*printerReadyDelaySec) * time.Second)
				continue
			}

			// set printerAttributes to be used later
			printerAttributes = printerAttrsResponse.PrinterAttributes
			//todo: raw values of PrinterAttributes may contain invalid UTF-8 chars which need to be handled by the caller when marshal data into JSON
			msg := "received supported printer attributes"
			pclog.Devf(msg)
			processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempts, msg, duration)
			break getPrinterAttrs
		}
	}
	return printerAttributes, nil
}

func operationsSupported(printerAttrs *ippclient.PrinterAttributes, requiredOps []ipp.Operation) bool {
	if len(requiredOps) == 0 {
		pclog.Errorf("no required operations provided. failing")
		return false
	}

	for _, op := range requiredOps {
		found := false
		for _, k := range printerAttrs.OperationsSupported {
			if k == int(op) {
				found = true
				break
			}
		}
		if !found {
			pclog.Devf("required operation %d not supported", op)
			return false
		}
	}

	return true
}

// Printer is ready if none of these conditions are met
//   - Printer is not accepting jobs
//   - Printer reports spool-area-full
func isPrinterReady(response *ippclient.PrinterAttributes) (bool, string) {
	if !response.PrinterIsAcceptingJobs {
		return false, "printer-not-accepting-jobs"
	}

	if isSpoolAreaFull(response.PrinterStateReasons) {
		return false, "spool-area-full"
	}

	return true, ""
}

func isSpoolAreaFull(printerStateReasons []string) bool {
	for _, printerStateReason := range printerStateReasons {
		if strings.Contains(printerStateReason, "spool-area-full") {
			return true
		}
	}
	return false
}

func makeIPPJobAttributes(ticketAttrs *jobticket.JobTicket, printerAttrs *ippclient.PrinterAttributes) *ippclient.PrintJobTemplateAttributes {
	mediaColSupported := printerAttrs.MediaColSupported != nil || len(printerAttrs.MediaColSupported) != 0

	jobAttrs := &ippclient.PrintJobTemplateAttributes{
		AttributeCopies: ticketAttrs.Copies,
		PrintColorMode:  ticketAttrs.PrintColorMode,
		AttributesSides: ticketAttrs.Sides,
		Finishings:      mapFinishings(ticketAttrs, printerAttrs),
		MultiDocHandle:  ippclient.SeparateDocumentsCollatedCopies,
	}

	mediaSize := getIppMediaSizeFromName(ticketAttrs.PaperName)
	if mediaColSupported {
		jobAttrs.MediaCol = map[string]interface{}{
			"media-size": map[string]interface{}{
				"x-dimension": int(mediaSize.Width),
				"y-dimension": int(mediaSize.Height),
			},
		}
	} else {
		jobAttrs.Media = mediaSize.Name
	}

	applyPdlOverrides(jobAttrs, ticketAttrs)

	return jobAttrs
}

// NOTE: map the document formats detected by analysis to a version supported by the printer
func mapDocumentFormat(ticketAttrs *jobticket.JobTicket, printerAttrs *ippclient.PrinterAttributes) string {
	spoolFormat := strings.TrimSpace(ticketAttrs.DocumentFormat)
	// Check whether the spooled doc format is in the printers list of supported document formats.
	for _, supportFormat := range printerAttrs.DocumentFormatSupported {
		if spoolFormat == strings.TrimSpace(supportFormat) {
			return supportFormat
		}
	}

	if len(ticketAttrs.AltDocumentFormat) > 0 {
		// Check whether any of the printers supported formats are in alternate document formats.
		// spooled doc format is in the printers list of supported document formats.
		for _, supportFormat := range printerAttrs.DocumentFormatSupported {
			for _, n := range ticketAttrs.AltDocumentFormat {
				if strings.TrimSpace(n) == strings.TrimSpace(supportFormat) {
					pclog.Devf("Using alternate document format (%s) for spooled format:%s", n, spoolFormat)
					return supportFormat
				}
			}
		}
	}

	// If we get here, we have no alternate doc format mapping for the printer & spooled doc format.
	// Any default doc format mapping will be the last resort.
	// Is it application/pdf : Then return as is.
	if spoolFormat == "application/pdf" {
		return spoolFormat
	}

	return ""
}

func mapFinishings(ticketAttrs *jobticket.JobTicket, printerAttrs *ippclient.PrinterAttributes) []int {
	var allFinishings []int

	for _, finishing := range ticketAttrs.Finishings {
		reqFinishingsEnum, ok := finishingsStringToEnumMap[finishing]
		if !ok {
			pclog.Errorf("Requested finishing %s is not supported by the ippclient. Update the finishings.FinishingsMap", finishing)
			continue
		}
		// Position of some finishing options change with orientation of document
		if ticketAttrs.OptionalPDLOverrides.Orientation == jobticket.OrientationLandscape {
			reqFinishingsEnum = evaluateFinishingsForLandscape(reqFinishingsEnum)
		}
		if finishingsEnum, ok := getSupportedFinishingsEnum(reqFinishingsEnum, printerAttrs.FinishingsSupported); ok {
			allFinishings = append(allFinishings, finishingsEnum)
		}
	}

	return allFinishings
}

func getSupportedFinishingsEnum(reqFinishingsEnum finishings.Finishings, finishingsSupported []int) (int, bool) {

	var supportedFinishingMap = make(map[int]struct{})
	for _, supportedFinishing := range finishingsSupported {
		supportedFinishingMap[supportedFinishing] = struct{}{}
	}

	if _, supported := supportedFinishingMap[int(reqFinishingsEnum)]; supported {
		return int(reqFinishingsEnum), true
	}

	genericFinishingOption, ok := genericFinishings[reqFinishingsEnum]
	if ok {
		if _, supported := supportedFinishingMap[int(genericFinishingOption)]; supported {
			pclog.Supportf("generic finishing type for %d is supported by the printer. Falling back to %d", reqFinishingsEnum, genericFinishingOption)
			return int(genericFinishingOption), true
		}
	}

	pclog.Supportf("Requested finishing %d is not supported by the printer, ignoring.", reqFinishingsEnum)
	return 0, false
}

func evaluateFinishingsForLandscape(enum finishings.Finishings) finishings.Finishings {
	result := enum
	switch enum {
	case finishings.FinishingsStapleTopLeft:
		result = finishings.FinishingsStapleBottomLeft
	case finishings.FinishingsStapleBottomLeft:
		result = finishings.FinishingsStapleBottomRight
	case finishings.FinishingsStapleTopRight:
		result = finishings.FinishingsStapleTopLeft
	case finishings.FinishingsStapleBottomRight:
		result = finishings.FinishingsStapleTopRight
	}
	if result != enum {
		pclog.Supportf("Finishings changed for landscape orientation input %d, mapped to %d", enum, result)
	}
	return result
}

func checkPrinter(printerURI string, httpClient ippclient.HttpClientInterface,
	attribCache *printerattributecache.PrinterAttributeCache,
	deviceId, deviceSnRegex string) error {

	startTime := time.Now()
	opeErr := &OperationError{
		Type: ErrCheckPrinter,
	}

	if printerURI == "" {
		usage()
		opeErr.Err = fmt.Errorf("printerURI empty")
		return opeErr
	}

	pclog.Supportf("get-printer-attributes:[%v] starting", printerURI)

	// Try to get the ipp-printer-attributes from cache.
	// This is a best effort only, if the printer data is not cached it's not an error.
	if attribCache != nil {
		printerAttrs, err := attribCache.GetPrinterAttributes(printerURI)
		if err == nil {
			// Log this, mainly for collecting stats and later adjusting the cache expiry etc.
			processingLogger.LogOperationAttempt(getPrinterAttrsOperation, 1, "ipp-printer-attribute-cache: Found", time.Since(startTime).String())
			if deviceId != "" && checkPrinterDeviceIdMatch(deviceId, deviceSnRegex, printerAttrs) {
				return nil
			} else {
				pclog.Devf("ipp-printer-attribute-cache: ipp-device-id attr doesn't match, will call out " +
					"to printer to get fresh attributes")
			}
		} else {
			processingLogger.LogOperationAttempt(getPrinterAttrsOperation, 1, "ipp-printer-attribute-cache: Not Found", time.Since(startTime).String())
			pclog.Devf("ipp-printer-attribute-cache: %v err: %v ", printerURI, err)
		}
	}

	client, err := ippclient.NewIPPClient(ippclient.SetHTTPClient(httpClient))
	if err != nil {
		opeErr.Err = fmt.Errorf("failed to create ipp client, err: %v", err)
		return opeErr
	}
	defer func() {
		err := client.Close()
		if err != nil {
			pclog.Errorf("failed to close IPP client: %v", err)
		}
	}()

	attempt := 0
	var printerAttrsResponse *ippclient.PrinterAttributesResponse
	// Note : start with 1, as the first run is attempt 1.
	// If we get malformed IPP attributes, return immediately.
	// Else, retry for *ippGetAttributeRetries times with a sleep in between.
	for attempt = 1; attempt <= *ippGetAttributeRetries; attempt++ {
		printerAttrsResponse, err = client.GetPrinterAttributes(printerURI, printerReadyAttributes)
		if err != nil {
			if err == ippclient.ErrMalformedAttributes {
				processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempt, "failed-malformed-attributes", time.Since(startTime).String())
				return &OperationError{
					Type: ErrCheckPrinterErrorResponse,
					Err: fmt.Errorf("get-printer-attributes:[%v] failed err: %v, elapsed:%v ",
						printerURI, err, time.Since(startTime)),
				}
			} else {
				// Log the errors to processing log. If we get killed by the os at ctx timeout, these won't be lost.
				es := fmt.Sprintf("failed err: %v, retrying", err)
				processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempt, es, time.Since(startTime).String())
				pclog.Errorf("get-printer-attributes:[%v] error attempt:%v err: %v, elapsed:%v",
					printerURI, attempt, err, time.Since(startTime))
				// Sleep for 500ms before retrying.
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		if !printerAttrsResponse.StatusCode.IsStatusOK() {
			processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempt, "failed-printer not ready", time.Since(startTime).String())
			return &OperationError{
				Type: ErrCheckPrinterPrinterNotReady,
				Err:  fmt.Errorf("get-printer-attributes:[%v] done, printer is not ready", printerURI),
			}
		}
		// If we come here, we have a valid response.
		break
	}

	// Failed with possibly a network related error. Try to get the route info and add it to the error.
	if err != nil || printerAttrsResponse == nil {
		routeInfo, _ := info.GetRoutingInfoForURI(printerURI)
		es := fmt.Sprintf("failed err: %v", err)
		processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempt-1, es, time.Since(startTime).String())
		return &OperationError{
			Type: ErrCheckPrinterNetwork,
			Err: fmt.Errorf("get-printer-attributes:[%v] failed err: %v, attempt:%v, elapsed:%v route[%v]",
				printerURI, err, attempt, time.Since(startTime), routeInfo),
		}
	}

	pclog.Supportf("get-printer-attributes:[%v] success, elapsed:%v", printerURI, time.Since(startTime))
	msg := fmt.Sprintf("Done:status code - %v", printerAttrsResponse.StatusCode)
	processingLogger.LogOperationAttempt(getPrinterAttrsOperation, attempt, msg, time.Since(startTime).String())

	if deviceId != "" && !checkPrinterDeviceIdMatch(deviceId, deviceSnRegex, printerAttrsResponse.PrinterAttributes) {
		return &OperationError{
			Type: ErrCheckPrinterDeviceIdMismatch,
			Err:  fmt.Errorf("printer device Id does not match the criteria"),
		}
	}
	// Update the attribute cache only if the printer is ready.
	// This info will be valid for a while (30sec). If ipp-print-client uses the same printer URI
	// during that time, the cached value will be used preventing reaching the printer.
	if attribCache != nil {
		if ready, _ := isPrinterReady(printerAttrsResponse.PrinterAttributes); ready {
			pclog.Devf("ipp-printer-attribute-cache: Saving printer attributes for: %v", printerURI)
			err = attribCache.SetPrinterAttributes(printerURI, printerAttrsResponse.PrinterAttributes)
		}
	}
	return nil
}

// Defaults to "true", "printer-device-id" check is best effort check.
func checkPrinterDeviceIdMatch(deviceIdRaw, deviceIdSnRegex string, attributes *ippclient.PrinterAttributes) bool {
	if deviceIdRaw != "" && attributes != nil {
		if attributes.PrinterDeviceID == deviceIdRaw {
			return true
		} else if deviceIdSnRegex != "" {
			var sn string
			var snFromPrinterAttrs string
			snRegex := regexp.MustCompile(deviceIdSnRegex)
			if snRegex.MatchString(deviceIdRaw) {
				snMatchDeviceIdRaw := snRegex.FindStringSubmatch(deviceIdRaw)
				sn = snMatchDeviceIdRaw[2]
			}
			if snRegex.MatchString(attributes.PrinterDeviceID) {
				snMatchPrinterDeviceId := snRegex.FindStringSubmatch(attributes.PrinterDeviceID)
				snFromPrinterAttrs = snMatchPrinterDeviceId[2]
			}
			return sn != "" && sn == snFromPrinterAttrs
		}
		return false
	}
	return true
}
