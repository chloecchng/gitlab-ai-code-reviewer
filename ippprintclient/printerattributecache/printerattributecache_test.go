package printerattributecache

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/gopapercut/print/ipp"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
	"bitbucket.org/papercutsoftware/gopapercut/random"
	utilconfig "bitbucket.org/papercutsoftware/pmitc-coordinator/util/config"
)

var L3230CDWIppAttribs = &ippclient.PrinterAttributes{
	CharsetConfigured:             "",
	CharsetSupported:              nil,
	ColorSupported:                false,
	CompressionSupported:          nil,
	DocumentFormatDefault:         "application/octet-stream",
	DocumentFormatSupported:       []string{"application/octet-stream", "image/urf", "image/jpeg", "image/pwg-raster"},
	IPPVersionsSupported:          []string{"1.0", "1.1", "2.0"},
	JobPasswordSupported:          0,
	MediaColSupported:             []string{"media-type", "media-size", "media-top-margin", "media-left-margin", "media-right-margin", "media-bottom-margin", "media-source", "media-auto-dimension", "media-source-propaties"},
	MultipleDocumentJobsSupported: false,
	MultipleOperationTimeout:      0,
	OperationsSupported:           []int{2, 4, 5, 6, 8, 9, 10, 11, 59, 60},
	FinishingsSupported:           []int{3},
	OrientationRequestedDefault:   0,
	PagePerMinute:                 0,
	PagesPerMinuteColor:           0,
	PrinterIsAcceptingJobs:        true,
	PrinterMakeModel:              "Brother HL-L3230CDW series",
	PrinterState:                  3,
	PrinterStateReasons:           []string{"none"},
	PrinterUptime:                 0,
	QueuedJobCount:                0,
	SidesSupported:                []string{"one-sided", "two-sided-long-edge", "two-sided-short-edge"},
	CopiesSupported:               ipp.RangeOfInteger{LowerBound: 1, UpperBound: 99},
	CopiesDefault:                 0,
	PrinterResolutionDefault: ipp.Resolution{
		XRes:  2,
		YRes:  20,
		Units: 254,
	},
}

func getPathToTempDir() (string, error) {

	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		err = fmt.Errorf("cloudn't create temporary directory %v", err)
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0770); err != nil {
		err := fmt.Errorf("failed to create cache directory err %v", err)
		return "", err
	}
	return tmpDir, nil
}

func Test_SaveFile(t *testing.T) {
	printerURI := []string{
		"ipps://10.50.20.54:631/ipp/print",
		"ipp://10.50.20.54:631/ipp/print",
		"ipps://10.50.20.69:631/ipp/print",
		"ipp://10.50.20.69:631/ipp/print",
		"ipp://192.168.1.12/ipp/print",
		"ipps://10.50.20.74:631/ipp/print",
		"ipp://10.50.20.74:631/ipp/print",
		"ipps://10.50.20.27:631/ipp/print",
		"ipp://10.50.20.27:631/ipp/print",
		"https://10.50.20.47/ipp/print",
		"https://10.50.20.47:631/ipp/print",
		"http://10.50.20.47/ipp/print",
		"http://10.50.20.47:631/ipp/print",
		"ipps://10.50.20.46:443/ipp/print",
		"ipp://10.50.20.46:631/ipp/print",
		"ipps://10.50.20.5/ipp",
		"https://10.50.20.5/ipp",
		"ipps://konica-c224e.papercutsoftware.com/ipp",
		"https://konica-c224e.papercutsoftware.com/ipp",
		"ipp://10.50.20.5/ipp",
		"ipps://10.50.20.69:631/ipp/print",
		"ipp://10.50.20.69:631/ipp/print",
		"ipps://10.50.20.25:443/ipp/print",
		"ipp://10.50.20.25:631/ipp/print",
		"ipps://10.50.20.46:443/ipp/print",
		"ipp://10.50.20.46:631/ipp/print",
		"http://192.168.0.25:631/ipp/print",
		"https://kyocera-250ci:443/printers/lp1",
		"https://kyocera-250ci:443/printers/lp2",
		"https://kyocera-250ci:443/printers/lp3",
		"https://kyocera-250ci:443/printers/lp4",
		"http://127.0.0.1:9363/ipp/print",
		"http://127.0.0.1:9363/ipp/print",
		"ipps://10.100.66.177:631/ipp/print",
		"ipp://10.100.66.177:631/ipp/print",
		"http://127.0.0.1:80/ipp/print",
		"http://127.0.0.1:80/ipp/print",
		"ipps://10.50.20.5/ipp",
		"https://10.50.20.5/ipp",
		"ipps://konica-c224e.papercutsoftware.com/ipp",
		"https://konica-c224e.papercutsoftware.com/ipp",
		"ipp://10.50.20.5/ipp",
		"ipp://10.50.20.48/ipp/print",
		"http://10.50.20.38:631/ipp",
		"http://10.50.20.38:631/ipp/lp",
		"https://10.50.20.61/ipp/print",
		"http://10.50.20.61:631/ipp/print",
		"http://10.50.20.61/ipp/print",
		"ipps://10.50.20.67:631/ipp/print",
		"ipp://10.50.20.67:631/ipp/print",
		"ipps://192.168.1.117/ipp/print",
		"ipp://192.168.1.117/ipp/print",
	}

	utilconfig.Initialise()

	tmpDir, err := getPathToTempDir()
	if err != nil {
		t.Fatalf("cloudn't create temporary directory %v", err)
	}
	fmt.Printf("temporary directory %v\n", tmpDir)
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("cloudn't remove temporary directory %v", err)
		}
	}()

	for _, uri := range printerURI {
		s := convertURItoFileName(uri)
		if len(s) == 0 {
			t.Fatalf("Failed to convert %v", uri)
		}
		filePath := filepath.Join(tmpDir, fmt.Sprintf(fileTemplate, convertURItoFileName(uri)))
		if filePath == "" {
			t.Fatalf("failed to create file tmpDir")
		}
		pac := &cacheElement{
			PrinterUri:    uri,
			IppAttributes: *L3230CDWIppAttribs,
		}
		fmt.Printf("Saving file=%v\n", filePath)
		err = writePrinterAttributesToFile(pac, filePath)
		if err != nil {
			t.Fatalf("writePrinterAttributesToFile(%v) Failed", err)
		}
	}
}

// Save the same ipp-attributes struct under several URI's.
// Read it back and compare with the original.
func Test_SetAndGetSanity(t *testing.T) {
	pclog.SetOutput(os.Stderr)
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("failed to create temp %v", err)
	}
	pc, err := NewCache(20, tmpDir)
	if err != nil {
		t.Fatalf("NewCache(%v) Failed", err)
	}

	printerURI := []string{
		"ipps://10.50.20.54:631/ipp/print",
		"ipp://10.50.20.54:631/ipp/print",
		"ipp://192.168.1.12/ipp/print",
		"ipps://10.50.20.74:631/ipp/print",
		"ipp://10.50.20.74:631/ipp/print",
		"https://10.50.20.47:631/ipp/print",
		"ipp://10.50.20.46:631/ipp/print",
		"ipps://10.50.20.5/ipp",
		"https://konica-c224e.papercutsoftware.com/ipp",
		"ipp://10.50.20.5/ipp",
		"ipps://10.50.20.25:443/ipp/print",
		"ipp://10.50.20.25:631/ipp/print",
		"https://kyocera-250ci:443/printers/lp1",
		"https://kyocera-250ci:443/printers/lp2",
		"http://127.0.0.1:80/ipp/print",
		"ipps://10.50.20.5/ipp",
		"https://10.50.20.5/ipp",
		"ipps://konica-c224e.papercutsoftware.com/ipp",
		"https://konica-c224e.papercutsoftware.com/ipp",
		"ipp://10.50.20.5/ipp",
		"ipp://192.168.1.117/ipp/print",
	}

	// Set some cache elements with random values in them, read and compare.
	for _, uri := range printerURI {
		pattribs := L3230CDWIppAttribs
		pattribs.PrinterUptime = random.IntRange(1, math.MaxInt32)
		pattribs.PrinterUptime = random.IntRange(1, math.MaxInt32)
		pattribs.PrinterResolutionDefault.Units = ipp.Units(random.IntRange(1, 255))

		if err := pc.SetPrinterAttributes(uri, pattribs); err != nil {
			t.Fatalf("SetPrinterAttributes(%v) Failed", err)
		}
		pa, err := pc.GetPrinterAttributes(uri)
		if err != nil {
			t.Fatalf("GetPrinterAttributes(%v) Failed", err)
		}
		if !reflect.DeepEqual(*pa, *pattribs) {
			t.Fatalf("Printer attributes doesn't match %+v != %+v", *pa, *pattribs)
		}
	}

	pc.Cleanup()
}

func Test_Sanity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("failed to create temp %v", err)
	}
	pc, err := NewCache(2, tmpDir)
	if err != nil {
		t.Fatalf("NewCache(%v) Failed", err)
	}

	// non-existent URI
	uri := "ipps://10.50.20.54:631/ipp/print-none-exist"
	_, err = pc.GetPrinterAttributes(uri)
	if err == nil {
		t.Fatalf("Expected error got none")
	}

	uri = "https://10.50.20.54:631/ipp/good-printer"
	err = pc.SetPrinterAttributes(uri, L3230CDWIppAttribs)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	pc.Cleanup()
	_ = os.RemoveAll(tmpDir)
}

// Save the same ipp-attributes struct under several URI's.
// Read it back and compare with the original.
func Test_CacheExpiry(t *testing.T) {
	pclog.SetOutput(os.Stderr)
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("failed to create temp %v", err)
	}
	pc, err := NewCache(2, tmpDir)
	if err != nil {
		t.Fatalf("NewCache(%v) Failed", err)
	}
	uri := "ipps://10.50.20.54:631/ipp/print"
	pattribs := L3230CDWIppAttribs

	// Set and wait for some time to expire.
	if err := pc.SetPrinterAttributes(uri, pattribs); err != nil {
		t.Fatalf("SetPrinterAttributes(%v) Failed", err)
	}

	// This shouldn't fail.
	_, err = pc.GetPrinterAttributes(uri)
	if err != nil {
		t.Fatalf("Didn't expect error but got %v", err)
	}

	// Sleep until the cache expire.
	time.Sleep(3 * time.Second)

	// Now the cache should be expired.
	_, err = pc.GetPrinterAttributes(uri)
	if err == nil || err != ErrCacheExpired {
		t.Fatalf("Expected error 'ErrCacheExpired' but %v", err)
	}

	pc.Cleanup()
	_ = os.RemoveAll(tmpDir)
}
