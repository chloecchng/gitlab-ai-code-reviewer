package ippprintclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

type readCloseResetter interface {
	io.ReadCloser
	Reset() (readCloseResetter, error)
}

type streamReader struct {
	io.ReadCloser
	isRead  bool
	tmpFile *os.File
}

func (s *streamReader) Read(b []byte) (int, error) {
	s.isRead = true
	return s.ReadCloser.Read(b)
}

func (s *streamReader) Reset() (readCloseResetter, error) {
	if !s.isRead {
		return s, nil
	}
	_, err := io.Copy(ioutil.Discard, s.ReadCloser)
	if err != nil {
		return nil, fmt.Errorf("failed to read document: %v", err)
	}

	_, err = s.tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to reset reader: %v", err)
	}
	return &fileReader{s.tmpFile}, nil
}

type fileReader struct {
	*os.File
}

func (f *fileReader) Reset() (readCloseResetter, error) {
	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to reset reader: %v", err)
	}

	return f, nil
}
