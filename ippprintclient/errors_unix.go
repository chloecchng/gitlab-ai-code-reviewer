//go:build darwin || linux
// +build darwin linux

package ippprintclient

import (
	"errors"
	"syscall"
)

func isTemporarySyscallErr(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED)
}
