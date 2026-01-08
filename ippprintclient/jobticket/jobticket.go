package jobticket

import (
	"encoding/json"
	"fmt"
	"os"
)

type PDLOverrides struct {
	Orientation OrientationType
}

type JobTicket struct {
	Copies               int
	PrintColorMode       string
	Sides                string
	DocumentFormat       string
	PaperName            string
	PaperWidthMM         int
	PaperHeightMM        int
	OptionalPDLOverrides PDLOverrides // Specifies which PDL overrides to apply to the print job. E.g. orientation, duplex, etc. Note, this doesn't specify the actual values to apply.
	Credentials          Credentials
	Finishings           []string
	AltDocumentFormat    []string // Printer specific alternate document formats (overrides to spooled type if required).
}

type Credentials struct {
	Username string
	Password string
}

func (t *JobTicket) validate() error {
	if t.PrintColorMode == "" {
		return fmt.Errorf("invalid color mode")
	}

	if t.Sides == "" {
		return fmt.Errorf("invalid value for sides")
	}

	if t.Copies < 1 {
		return fmt.Errorf("invalid number of copies")
	}

	if t.PaperName == "" || t.PaperHeightMM == 0 || t.PaperWidthMM == 0 {
		return fmt.Errorf("invalid paper name, height or width values")
	}

	if t.DocumentFormat == "" {
		return fmt.Errorf("invalid document format")
	}

	return nil
}

type OrientationType string

const (
	OrientationPortrait  OrientationType = "portrait"
	OrientationLandscape OrientationType = "landscape"
)

func read(path string) (*JobTicket, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ticket JobTicket
	err = json.Unmarshal(data, &ticket)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}

func GetPrintJobAttributes(ticketPath string) (*JobTicket, error) {
	jobTicket, err := read(ticketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read job ticket file: %v. error: %v", ticketPath, err)
	}

	if err := jobTicket.validate(); err != nil {
		return nil, fmt.Errorf("invalid job ticket: %v", err)
	}

	return jobTicket, nil
}
