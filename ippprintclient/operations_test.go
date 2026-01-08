package ippprintclient

import (
	"testing"

	"bitbucket.org/papercutsoftware/gopapercut/print/ipp"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3/finishings"
	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient/jobticket"
)

func TestMapFinishings_Ok(t *testing.T) {
	jobTicket := &jobticket.JobTicket{
		Finishings: []string{"staple-top-left", "fold-half"},
	}
	printerAttrs := &ippclient.PrinterAttributes{
		FinishingsSupported: []int{20, 93},
	}
	mappedFinishings := mapFinishings(jobTicket, printerAttrs)

	if len(mappedFinishings) != 2 ||
		(mappedFinishings[0] != 20 && mappedFinishings[0] != 93) ||
		(mappedFinishings[1] != 20 && mappedFinishings[1] != 93) {
		t.Fatalf("mappedFinishings were not mapped properly. output %v, expected 20, 93", mappedFinishings)
	}
}

func TestMapFinishings_GenericMapping_OK(t *testing.T) {
	jobTicket := &jobticket.JobTicket{
		Finishings: []string{"staple-top-left"}, // staple-top-left = 20
		OptionalPDLOverrides: jobticket.PDLOverrides{
			Orientation: jobticket.OrientationLandscape,
		},
	}
	printerAttrs := &ippclient.PrinterAttributes{
		FinishingsSupported: []int{int(finishings.FinishingsStapleTopLeft), int(finishings.FinishingsStaple)},
	}
	mappedFinishings := mapFinishings(jobTicket, printerAttrs)

	// For landscape finishing option must be rotated 90 degrees anti clockwise
	// But Bottom left in this case is not supported by the printer and a generic staple is supported.
	// We should get the generic as the mapping.
	if len(mappedFinishings) != 1 || mappedFinishings[0] != int(finishings.FinishingsStaple) {
		t.Fatalf("mappedFinishings were not mapped properly. output %v, expected %v", mappedFinishings, finishings.FinishingsStapleBottomLeft)
	}
}

func TestMapFinishings_GenericMapping_NotSupported(t *testing.T) {
	jobTicket := &jobticket.JobTicket{
		Finishings: []string{"staple-top-left"}, // staple-top-left = 20
		OptionalPDLOverrides: jobticket.PDLOverrides{
			Orientation: jobticket.OrientationLandscape,
		},
	}
	printerAttrs := &ippclient.PrinterAttributes{
		FinishingsSupported: []int{int(finishings.FinishingsStapleTopLeft)},
	}
	mappedFinishings := mapFinishings(jobTicket, printerAttrs)

	// For landscape finishing option must be rotated 90 degrees anti clockwise
	// But Bottom left in this case is not supported by the printer and a no generic staple is supported.
	// We shouldn't get a finishings mapping.
	if len(mappedFinishings) != 0 {
		t.Fatalf("mappedFinishings were not mapped properly. Got %v, non expected", mappedFinishings)
	}
}

func TestMapFinishings_Portrait_OK(t *testing.T) {
	jobTicket := &jobticket.JobTicket{
		Finishings: []string{"staple-top-left"}, // staple-top-left = 20
		OptionalPDLOverrides: jobticket.PDLOverrides{
			Orientation: jobticket.OrientationPortrait,
		},
	}
	printerAttrs := &ippclient.PrinterAttributes{
		FinishingsSupported: []int{int(finishings.FinishingsStapleTopLeft), int(finishings.FinishingsStapleBottomLeft)},
	}
	mappedFinishings := mapFinishings(jobTicket, printerAttrs)

	if len(mappedFinishings) != 1 || mappedFinishings[0] != int(finishings.FinishingsStapleTopLeft) {
		t.Fatalf("mappedFinishings were not mapped properly. output %v, expected %v", mappedFinishings, finishings.FinishingsStapleTopLeft)
	}
}

func TestMapFinishings_Landscape_OK(t *testing.T) {
	jobTicket := &jobticket.JobTicket{
		Finishings: []string{"staple-top-left"}, // staple-top-left = 20
		OptionalPDLOverrides: jobticket.PDLOverrides{
			Orientation: jobticket.OrientationLandscape,
		},
	}
	printerAttrs := &ippclient.PrinterAttributes{
		FinishingsSupported: []int{int(finishings.FinishingsStapleTopLeft), int(finishings.FinishingsStapleBottomLeft)},
	}
	mappedFinishings := mapFinishings(jobTicket, printerAttrs)

	// For landscape finishing option must be rotated 90 degrees anti clockwise
	if len(mappedFinishings) != 1 || mappedFinishings[0] != int(finishings.FinishingsStapleBottomLeft) {
		t.Fatalf("mappedFinishings were not mapped properly. output %v, expected %v", mappedFinishings, finishings.FinishingsStapleBottomLeft)
	}
}

func TestOperationsSupported_V11NotSupported(t *testing.T) {
	tt := map[string]*ippclient.PrinterAttributes{
		"no-send-doc": {
			OperationsSupported: []int{int(ipp.OperationCreateJob), int(ipp.OperationPrintJob)},
		},
		"ipp-v10": {
			OperationsSupported: []int{int(ipp.OperationPrintJob)},
		},
		"no-supported-ops": {
			OperationsSupported: nil,
		},
	}

	for name, tc := range tt {
		if operationsSupported(tc, []ipp.Operation{ipp.OperationCreateJob, ipp.OperationSendDocument}) {
			t.Fatalf("expected required operations to be not supported for test %s", name)
		}
	}
}

func TestOperationsSupported_V11Supported(t *testing.T) {
	printerAttrs := &ippclient.PrinterAttributes{
		OperationsSupported: []int{int(ipp.OperationCreateJob), int(ipp.OperationSendDocument)},
	}

	if !operationsSupported(printerAttrs, []ipp.Operation{ipp.OperationCreateJob, ipp.OperationSendDocument}) {
		t.Fatalf("expected required operations to be supported")
	}
}

// A list of tests for document format mappings.
func TestMapDocumentFormats_Sanity(t *testing.T) {

	// Test : General case - no special printer based formats.
	// Incoming job ticket.
	ticketAttrs := &jobticket.JobTicket{
		Copies:            1,
		DocumentFormat:    "application/vnd.hp-PCLXL",
		AltDocumentFormat: []string{"application/pcl6", "application/pcl", "application/vnd.hp-PCL", "application/octet-stream"},
	}

	// Printer supports.
	printerAttrs := &ippclient.PrinterAttributes{
		DocumentFormatSupported: []string{"application/pcl6", "application/octet-stream", "application/postscript"},
	}

	ret := mapDocumentFormat(ticketAttrs, printerAttrs)
	if ret != "application/pcl6" {
		t.Fatalf("expected document format to be application/pcl6, got %s", ret)
	}

	// Test : for any printer with application/pdf supported, Spooled format = application/pdf,
	// Printing format should always be = "application/pdf"
	printerAttrs.DocumentFormatSupported = []string{"application/octet-stream", "application/pdf"}
	ticketAttrs.DocumentFormat = "application/pdf"
	ticketAttrs.AltDocumentFormat = []string{"application/alt-doc-fmt"}
	ret = mapDocumentFormat(ticketAttrs, printerAttrs)
	if ret != "application/pdf" {
		t.Fatalf("expected document format to be application/pdf, got %s", ret)
	}

	// spooled : "application/postscript", alt-fmt:"application/octet-stream" -> printing: "application/octet-stream"
	printerAttrs.DocumentFormatSupported = []string{"application/octet-stream", "image/urf"}
	printerAttrs.PrinterMakeModel = "TOSHIBA e-STUDIO3515AC"
	ticketAttrs.DocumentFormat = "application/postscript"
	ticketAttrs.AltDocumentFormat = []string{"application/octet-stream"}
	ret = mapDocumentFormat(ticketAttrs, printerAttrs)
	if ret != "application/octet-stream" {
		t.Fatalf("expected document format to be application/octet-stream, got %s", ret)
	}

	// For Any Printer, with application/vnd.hp-PCL supported, spooled:"application/vnd.hp-PCL" -> Printing: application/vnd.hp-PCL
	// regardless of the alt-fmts
	printerAttrs.DocumentFormatSupported = []string{"application/vnd.hp-PCL", "application/octet-stream", "application/pcl"}
	printerAttrs.PrinterMakeModel = "printer-JRHU44Wgx series"
	ticketAttrs.DocumentFormat = "application/vnd.hp-PCL"
	ticketAttrs.AltDocumentFormat = []string{"application/octet-stream"}
	ret = mapDocumentFormat(ticketAttrs, printerAttrs)
	if ret != "application/vnd.hp-PCL" {
		t.Fatalf("expected document format to be application/pcl, got %s", ret)
	}

	// For Any Printer, with "application/pcl6", "application/pcl", "application/vnd.hp-PCL", "application/octet-stream"
	// supported, spooled: "application/vnd.hp-PCLXL" -> out the first one from the list DocumentFormatSupported.
	printerAttrs.DocumentFormatSupported = []string{"application/pcl6", "application/pcl", "application/vnd.hp-PCL", "application/octet-stream"}
	printerAttrs.PrinterMakeModel = "printer-JRHU44Wgx series"
	ticketAttrs.DocumentFormat = "application/vnd.hp-PCLXL"
	ticketAttrs.AltDocumentFormat = []string{"application/pcl6", "application/pcl", "application/vnd.hp-PCL", "application/octet-stream"}
	ret = mapDocumentFormat(ticketAttrs, printerAttrs)
	if ret != "application/pcl6" {
		t.Fatalf("expected document format to be application/pcl6, got %s", ret)
	}

}

func TestCheckPrinterDeviceIdMatch_Sanity(t *testing.T) {
	deviceIdRaw := "MFG:FUJIFILM;CMD:PJL,RASTER,DOWNLOAD,HBPL,PCLXL,PCL,POSTSCRIPT,URF;URF:CP255,DM1,FN3,IFU0,IS1-4-20,MT1-3-4-5-6,OB10,PQ4,RS600,SRGB24,V1.4,W8;SN:TR4-000491;MDL:Apeos C325z/328df;CID:FF_PCL_COLOR;DES:FF AC325z;CLS:PRINTER;"
	snRegex := "(SN|SER):(.*?)(;|$)"
	testPrinterAttrs := &ippclient.PrinterAttributes{
		PrinterDeviceID: deviceIdRaw,
	}
	if !checkPrinterDeviceIdMatch(deviceIdRaw, snRegex, testPrinterAttrs) {
		t.Fatalf("expected passed deviceIdRaw and ippclient.PrinterAttributes.PrinterDeviceID to match")
	}
}

func TestCheckPrinterDeviceIdMatch_SnMatch(t *testing.T) {
	deviceIdRaw := "SN:TR4-000491"
	printerAttrsDeviceIdRaw := "MFG:FUJIFILM;CMD:PJL,RASTER,DOWNLOAD,HBPL,PCLXL,PCL,POSTSCRIPT,URF;URF:CP255,DM1,FN3,IFU0,IS1-4-20,MT1-3-4-5-6,OB10,PQ4,RS600,SRGB24,V1.4,W8;SN:TR4-000491;MDL:Apeos C325z/328df;CID:FF_PCL_COLOR;DES:FF AC325z;CLS:PRINTER;"
	snRegex := "(SN|SER):(.*?)(;|$)"
	testPrinterAttrs := &ippclient.PrinterAttributes{
		PrinterDeviceID: printerAttrsDeviceIdRaw,
	}
	if !checkPrinterDeviceIdMatch(deviceIdRaw, snRegex, testPrinterAttrs) {
		t.Fatalf("expected passed deviceIdRaw and ippclient.PrinterAttributes.PrinterDeviceID to match")
	}
}
func TestCheckPrinterDeviceIdMatch_NoMatch(t *testing.T) {
	deviceIdRaw := "MFG:FUJIFILM;CMD:PJL,RASTER,DOWNLOAD,HBPL,PCLXL,PCL,POSTSCRIPT,URF;URF:CP255,DM1,FN3,IFU0,IS1-4-20,MT1-3-4-5-6,OB10,PQ4,RS600,SRGB24,V1.4,W8;SN:TR4-000491;MDL:Apeos C325z/328df;CID:FF_PCL_COLOR;DES:FF AC325z;CLS:PRINTER;"
	printerAttrsDeviceIdRaw := "MFG:FUJIFILM;"
	snRegex := "(SN|SER):(.*?)(;|$)"
	testPrinterAttrs := &ippclient.PrinterAttributes{
		PrinterDeviceID: printerAttrsDeviceIdRaw,
	}
	if checkPrinterDeviceIdMatch(deviceIdRaw, snRegex, testPrinterAttrs) {
		t.Fatalf("expected passed deviceIdRaw and ippclient.PrinterAttributes.PrinterDeviceID to not match")
	}
}
