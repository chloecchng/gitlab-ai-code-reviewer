package ippprintclient

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bitbucket.org/papercutsoftware/gopapercut/httputils"
	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient/printerattributecache"
)

const (
	defaultHttpRequestTimeoutSec = 30
	defaultIPPCommandTimoutSec   = 30
	processingReportMarker       = "PROCESSING REPORT:"
)

var (
	ticketPath                              = flag.String("ticketPath", "", "job ticket path")
	printerURI                              = flag.String("printerURI", "", "printer uri")
	printerReadyTimeoutSec                  = flag.Int("printerReadyTimeoutSec", 600, "number of seconds before timing out")
	ippMaxPrintJobSendDocumentRetryAttempts = flag.Int("ippMaxPrintJobSendDocumentRetryAttempts", 5, "max number of retries for sen-document and print-job operations")
	printerReadyDelaySec                    = flag.Int("printerReadyDelaySec", 2, "min number of seconds to wait between attempts")
	help                                    = flag.Bool("help", false, "prints help")
	httpRequestTimeoutSec                   = flag.Int("httpRequestTimeoutSec", defaultHttpRequestTimeoutSec, "http client request timeout")
	httpConnectTimeoutSec                   = flag.Int("httpConnectTimeoutSec", 0, "http client connect timeout. If 0, Default TransportOptions from httputils will be used")
	httpResponseHeaderTimeoutSec            = flag.Int("httpResponseHeaderTimeoutSec", 0, "http client response header timeout. If 0, Default TransportOptions from httputils will be used")
	httpTlsHandshakeTimeoutSec              = flag.Int("httpTlsHandshakeTimeoutSec", 0, "http client tls handshake timeout. If 0, Default TransportOptions from httputils will be used")
	ippPrintOperation                       = flag.String("ippPrintOperation", "", "preferred ipp print operation")
	ippMaxUnauthorisedAttempts              = flag.Int("ippMaxUnauthorisedAttempts", 4, "maximum attempts to print when a printer returns unauthorised response (default matches iOS CUPS implementation)")
	maxCreateJobAttempts                    = flag.Int("maxCreateJobAttempts", 3, "maximum attempts to create a valid job")
	ippPrintDoc                             = flag.String("ippPrintDoc", "", "path to file to be printed, if not specified, stdin is used")
	printerAttributeCacheEnabled            = flag.Bool("printerAttributeCacheEnabled", false, "enable the printer attributes cache")
	printerAttributeCachePath               = flag.String("printerAttributeCachePath", "", "Path to printer attributes cache directory")
	ippCommandTimeoutSec                    = flag.Int("ippCommandTimeout", defaultIPPCommandTimoutSec, "Total time to finish the ipp command")
	ippGetAttributeRetries                  = flag.Int("ippGetAttributeRetries", 5, "max number of retries for get-attributes operations")
	ippDeviceId                             = flag.String("ippDeviceId", "", "ipp device id raw value")
	ippDeviceIdSnRegex                      = flag.String("ippDeviceIdSnRegex", "", "ipp device id serial number reg exp")
)

// General Exit codes returned by this executable
// See operations.go for operation specific error exit codes.
var (
	ExitCodeSuccess      int = 0
	ExitCodeErrorDefault int = 1 // Default failure exit code.
	ExitCodeHelp         int = 2 // Usage/help function exit code
)

func usage() {
	exeName := filepath.Base(os.Args[0])
	_, _ = fmt.Fprintf(os.Stdout,
		`usage: %s [flags] [check-printer|print-job]
	where [flags]:
		-ticketPath - path to job ticket
		-printerURI - printer uri
		-printerReadyTimeoutSec - timeout
		-printerReadyDelaySec - delay
		-httpRequestTimeoutSec - http client request timeout
		-httpConnectTimeoutSec - http client connect timeout
		-httpResponseHeaderTimeoutSec - http client response header timeout
		-httpTlsHandshakeTimeoutSec - http client tls handshake timeout
		-printerAttributeCacheEnabled - enable the printer attributes cache
		-printerAttributeCachePath - printer attributes cache will use this directory to store cache files
		-ippCommandTimeout - total time to finish the ipp command
		-ippDeviceId - ipp device id raw value
		-ippDeviceIdSnRegex - ipp device id serial number reg exp

	usage (test mode): %s -test -op [operation] -uri[printer uri]|-address[printer address] [flags]
	where [operation]: \get-printer-attributes\|\print-job\|\cups-get-printers\|\get-job-attributes\
	where [flags]:
		-job-id - job id
		-stdin - StandardIn - file input method
		-path - Path - file input method
		-media-size - paper size
		-document-format - format of the document provided (application/pdf, application/postscript, image/urf)`+"\n", exeName, exeName)
	os.Exit(ExitCodeHelp)
}

func Main() {
	flag.Parse()
	pclog.SetOutput(os.Stderr)
	startTime := time.Now()
	flag.Usage = usage

	var cmd string
	if len(flag.Args()) > 0 {
		cmd = flag.Args()[0]
	}

	if *help {
		flag.PrintDefaults()
		os.Exit(ExitCodeSuccess)
	}

	//setting up processing logger
	processingLogger = &ippclientProcessingLogger{
		output: os.Stderr,
	}

	httpTransportOptions := httputils.NewTransportOptions{
		TLSMinVersion:         "VersionTLS10",
		TLSInsecureSkipVerify: true,
	}

	if *httpConnectTimeoutSec > 0 {
		// Timeout is the maximum amount of time the TCP dial will wait for connect to complete.
		httpTransportOptions.ConnectTimeout = time.Duration(*httpConnectTimeoutSec) * time.Second
	}

	if *httpResponseHeaderTimeoutSec > 0 {
		// ResponseHeaderTimeout, specifies the amount of time to wait for a server's response headers after fully writing the
		// request (including its body, if any). This time does not include the time to read the response body.
		httpTransportOptions.ResponseHeaderTimeout = time.Duration(*httpResponseHeaderTimeoutSec) * time.Second
	}

	if *httpTlsHandshakeTimeoutSec > 0 {
		// TLSHandshakeTimeout specifies the maximum amount of time waiting to wait for a TLS handshake.
		httpTransportOptions.TLSHandshakeTimeout = time.Duration(*httpTlsHandshakeTimeoutSec) * time.Second
	}

	transport, err := httputils.NewTransport(httpTransportOptions)

	if err != nil {
		pclog.Errorf("failed to create http client, err: %v", err)
		os.Exit(ExitCodeErrorDefault)
	}

	httpReqTimeoutSec := *httpRequestTimeoutSec
	//RequestTimeoutSec can't be 0 as per defaultHttpClientWithSkipVerify from httpclient
	if *httpRequestTimeoutSec == 0 {
		httpReqTimeoutSec = defaultHttpRequestTimeoutSec
	}

	httpClient := &http.Client{
		// Timeout specifies a time limit for requests made by this Client. The timeout includes connection time, any
		// redirects, and reading the response body.
		Timeout:   time.Duration(httpReqTimeoutSec) * time.Second,
		Transport: transport,
	}

	var printerAttributeCache *printerattributecache.PrinterAttributeCache = nil

	if *printerAttributeCacheEnabled && *printerAttributeCachePath != "" {
		printerAttributeCache, err = printerattributecache.NewCache(30, *printerAttributeCachePath)
		// If we failed to initialise the cache, still continue without it, don't fail printing.
		if err != nil {
			pclog.Errorf(err.Error())
			printerAttributeCache = nil
		}
	}

	// IPP Get Attribute Retries can't be 0 or less.
	if *ippGetAttributeRetries <= 0 {
		pclog.Devf("ippGetAttributeRetries can't be 0 or less, setting it to 1")
		*ippGetAttributeRetries = 1
	}

	switch cmd {
	case "check-printer":
		pclog.Supportf("ippDeviceId: %v", *ippDeviceId)
		pclog.Supportf("ippDeviceIdSnRegex: %v", *ippDeviceIdSnRegex)
		err = checkPrinter(*printerURI, httpClient, printerAttributeCache, *ippDeviceId, *ippDeviceIdSnRegex)
	case "print-job":
		if *ippPrintDoc != "" {
			f, openErr := os.Open(*ippPrintDoc)
			if openErr != nil {
				pclog.Errorf("cannot open input file %v", openErr)
				os.Exit(1)
			}
			defer func() { _ = f.Close() }()
			err = printJob(*ticketPath, *printerURI, f, httpClient, printerAttributeCache, time.Duration(*ippCommandTimeoutSec)*time.Second)
		} else {
			err = printJob(*ticketPath, *printerURI, os.Stdin, httpClient, printerAttributeCache, time.Duration(*ippCommandTimeoutSec)*time.Second)
		}
	default:
		flag.PrintDefaults()
	}

	if err != nil {
		pclog.Errorf("ipp command:%v failed: %v", cmd, err)

		var opErr *OperationError
		// Handle operation errors.
		if errors.As(err, &opErr) {
			os.Exit(opErr.Type)
		}
		os.Exit(ExitCodeErrorDefault)
	} else {
		processingLogger.LogOperationAttempt(cmd, 1, "command execution success", time.Since(startTime).String())
	}
}
