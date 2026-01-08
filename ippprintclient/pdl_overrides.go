package ippprintclient

import (
	"bitbucket.org/papercutsoftware/gopapercut/pclog"
	"bitbucket.org/papercutsoftware/gopapercut/print/ipp"
	"bitbucket.org/papercutsoftware/gopapercut/print/ippclient/v3"
	"bitbucket.org/papercutsoftware/pmitc-coordinator/ippprintclient/jobticket"
)

func applyPdlOverrides(jobAttrs *ippclient.PrintJobTemplateAttributes, ticket *jobticket.JobTicket) {

	if ticket.OptionalPDLOverrides.Orientation != "" {
		pclog.Devf("orientation override requested: %s", ticket.OptionalPDLOverrides.Orientation)
		jobAttrs.OrientationRequested = ippOrientation(ticket.OptionalPDLOverrides.Orientation)
	}
}

func ippOrientation(orientation jobticket.OrientationType) ippclient.Orientation {
	switch orientation {
	case jobticket.OrientationPortrait:
		return ippclient.OrientationPortrait
	case jobticket.OrientationLandscape:
		return ippclient.OrientationLandscape
	}

	return ipp.Integer{}
}
