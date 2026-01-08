package ippprintclient

import "bitbucket.org/papercutsoftware/ipp/v2"

var defaultIppMediaSize = ipp.MediaTypeIsoA4

var ippMediaSizeMap = map[string]ipp.MediaType{
	"5x7":       ipp.MediaTypeNa5X7,
	"8x10":      ipp.MediaTypeNa8X10,
	"Legal":     ipp.MediaTypeNaLegal,
	"Letter":    ipp.MediaTypeNaLetter,
	"Invoice":   ipp.MediaTypeInvoice,
	"Executive": ipp.MediaTypeExecutive,
	"Foolscap":  ipp.MediaTypeFolio,
	"Ledger":    ipp.MediaTypeLedger,

	"A0":  ipp.MediaTypeIsoA0,
	"A1":  ipp.MediaTypeIsoA1,
	"A2":  ipp.MediaTypeIsoA2,
	"A3":  ipp.MediaTypeIsoA3,
	"A4":  ipp.MediaTypeIsoA4,
	"A5":  ipp.MediaTypeIsoA5,
	"A6":  ipp.MediaTypeIsoA6,
	"A7":  ipp.MediaTypeIsoA7,
	"A8":  ipp.MediaTypeIsoA8,
	"A9":  ipp.MediaTypeIsoA9,
	"A10": ipp.MediaTypeIsoA10,

	"ISO B0":  ipp.MediaTypeIsoB0,
	"ISO B1":  ipp.MediaTypeIsoB1,
	"ISO B2":  ipp.MediaTypeIsoB2,
	"ISO B3":  ipp.MediaTypeIsoB3,
	"ISO B4":  ipp.MediaTypeIsoB4,
	"ISO B5":  ipp.MediaTypeIsoB5,
	"ISO B6":  ipp.MediaTypeIsoB6,
	"ISO B7":  ipp.MediaTypeIsoB7,
	"ISO B8":  ipp.MediaTypeIsoB8,
	"ISO B9":  ipp.MediaTypeIsoB9,
	"ISO B10": ipp.MediaTypeIsoB10,

	"B0":  ipp.MediaTypeJisB0,
	"B1":  ipp.MediaTypeJisB1,
	"B2":  ipp.MediaTypeJisB2,
	"B3":  ipp.MediaTypeJisB3,
	"B4":  ipp.MediaTypeJisB4,
	"B5":  ipp.MediaTypeJisB5,
	"B6":  ipp.MediaTypeJisB6,
	"B7":  ipp.MediaTypeJisB7,
	"B8":  ipp.MediaTypeJisB8,
	"B9":  ipp.MediaTypeJisB9,
	"B10": ipp.MediaTypeJisB10,
}

func getIppMediaSizeFromName(name string) ipp.MediaType {
	if val, ok := ippMediaSizeMap[name]; ok {
		return val
	}
	return defaultIppMediaSize
}
