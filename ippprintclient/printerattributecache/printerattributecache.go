package printerattributecache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
	utilconfig "bitbucket.org/papercutsoftware/pmitc-coordinator/util/config"
	atomicwrite "github.com/natefinch/atomic"
)

const fileTemplate = "%s.ipp.attributes"
const cacheDir = "ipp-printer-attribute-cache"

type cacheElement struct {
	PrinterUri    string                      `json:"printer-uri"`
	IppAttributes ippclient.PrinterAttributes `json:"ipp-attributes"`
}

var ErrCacheExpired = errors.New("cache element expired")
var ErrNotExist = errors.New("file doesn't exist")
var ErrCacheUninitialised = errors.New("uninitialised cache")

type PrinterAttributeCache struct {
	cacheDir    string
	cacheExpiry time.Duration
}

// NewCache Get a new IPP printer cache.
// expiry - Cache expiry duration in Seconds.
// Note : This is backed by a directory /path/ipp-printerinfo-cache,
// Multiple instances of the PrinterAttributeCache could access the same dir.
func NewCache(expirySec uint, path string) (*PrinterAttributeCache, error) {
	pc := &PrinterAttributeCache{}
	pc.cacheExpiry = time.Duration(expirySec) * time.Second
	if expirySec == 0 {
		return nil, fmt.Errorf("invalid cache expiry duration %v", expirySec)
	}
	err := pc.Initialise(path)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

// Cleanup This is a cleanup function to wipe the cache directory, so far, mostly used in testing.
// In production, the cache will be setup in the ../data/job-processor/tmp
// which will be cleaned up at the next reboot.
func (i *PrinterAttributeCache) Cleanup() {
	if i == nil {
		return
	}
	_ = os.RemoveAll(i.cacheDir)
}

func (i *PrinterAttributeCache) Initialise(path string) error {

	if i == nil {
		return ErrCacheUninitialised
	}
	fullPath := filepath.Join(path, cacheDir)
	if err := os.MkdirAll(fullPath, utilconfig.DefaultFolderPermission); err != nil {
		err := fmt.Errorf("failed to create cache directory err %v", err)
		return err
	}
	i.cacheDir = fullPath

	return nil
}

// SetPrinterAttributes Set the printer attributes to cache for the given URI.
func (i *PrinterAttributeCache) SetPrinterAttributes(uri string, attributes *ippclient.PrinterAttributes) error {

	if i == nil {
		return ErrCacheUninitialised
	}
	if i.cacheDir == "" {
		return fmt.Errorf("ipp-printer-attribute-cache: un-initialised cache")
	}
	if uri == "" || attributes == nil {
		return fmt.Errorf("ipp-printer-attribute-cache: invalid parameters")
	}
	filePath := filepath.Join(i.cacheDir, fmt.Sprintf(fileTemplate, convertURItoFileName(uri)))
	if filePath == "" {
		return fmt.Errorf("failed to create file path")
	}

	printer := &cacheElement{
		PrinterUri:    uri,
		IppAttributes: *attributes,
	}
	return writePrinterAttributesToFile(printer, filePath)
}

// GetPrinterAttributes Get the printer attributes from cache for the given URI.
// Return nil if failure, cache expired or not found.
func (i *PrinterAttributeCache) GetPrinterAttributes(uri string) (*ippclient.PrinterAttributes, error) {

	if i == nil {
		return nil, ErrCacheUninitialised
	}
	if i.cacheDir == "" {
		return nil, fmt.Errorf("ipp-printer-attribute-cache: un-initialised cache")
	}
	if uri == "" {
		return nil, fmt.Errorf("ipp-printer-attribute-cache: invalid uri")
	}

	filePath := filepath.Join(i.cacheDir, fmt.Sprintf(fileTemplate, convertURItoFileName(uri)))
	if filePath == "" {
		return nil, fmt.Errorf("failed to create file path")
	}

	expired, err := i.isExpired(filePath)
	if err != nil {
		return nil, fmt.Errorf("getPrinterAttributes failed %v, %v", filePath, err)
	}
	if expired {
		return nil, ErrCacheExpired
	}

	cacheElem, err := readPrinterAttributesFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("getPrinterAttributes: failed to read file %v, %v",
			filePath, err)
	}

	if cacheElem.PrinterUri != uri {
		return nil, fmt.Errorf("getPrinterAttributes: cache element URI mispatch in=%v, element uri=%v",
			uri, cacheElem.PrinterUri)
	}

	return &cacheElem.IppAttributes, nil
}

// returns whether the cache element is expired.
// Return values:
//
//		Non-existing file, true + ErrNotExist.
//	 Failed to read file: true + error.
//	 File is expired: true + nil.
//	 File is not expired: false + nil.
func (i *PrinterAttributeCache) isExpired(path string) (bool, error) {

	if i == nil {
		// This is a programming error. expired or not doesn't matter.
		return false, ErrCacheUninitialised
	}
	info, err := os.Stat(path)

	// Non-existing file, consider as expired, this makes
	//	if expired { create/overwrite cache } block would include non-existing case as well.
	if errors.Is(err, os.ErrNotExist) {
		return true, ErrNotExist
	}
	// Failed to read the file, consider it as expired.
	if err != nil {
		err = fmt.Errorf("ipp-printer-attribute-cache: failed to read file %v, %v", path, err)
		return true, err
	}

	modtime := info.ModTime() // Last modified time
	modtime = modtime.Add(i.cacheExpiry)
	now := time.Now()

	// is 'now' after the is file's (last access time + cache expiry duration) ?
	if now.After(modtime) {
		return true, nil
	} else {
		return false, nil
	}
}

func readPrinterAttributesFromFile(path string) (*cacheElement, error) {

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var attrbs cacheElement
	err = json.Unmarshal(data, &attrbs)
	if err != nil {
		return nil, err
	}

	return &attrbs, nil
}

func writePrinterAttributesToFile(attrbs *cacheElement, path string) error {

	b, err := json.Marshal(attrbs)
	if err != nil {
		return err
	}

	// WriteFile atomically writes the contents of b to the specified file path.
	// an error occurs, the target file is guaranteed to be either fully written, or not written at all.
	// Writefile writes to a temp file and do an atomic replace which guarantees to either replace the
	// target file entirely, or not change either file.
	// This should avoid race conditions when multiple instances of the process writing/reading
	// to/from the same file.
	err = atomicwrite.WriteFile(path, bytes.NewReader(b))
	return err
}

// convertURItoFileName Generate a file name from URI
func convertURItoFileName(uri string) string {
	// Replace characters which are not allowed in a filename with UNDERSCOREs
	uniqueName := strings.ReplaceAll(uri, "://", "_")
	uniqueName = strings.ReplaceAll(uniqueName, ":", "_")
	uniqueName = strings.ReplaceAll(uniqueName, ".", "_")
	uniqueName = strings.ReplaceAll(uniqueName, "/", "_")
	return uniqueName
}
