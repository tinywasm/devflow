package devflow

import (
	"errors"
)

type MarkDown struct {
	rootDir     string
	destination string
	// input sources (one of these should be set before calling Extract)
	inputPath string

	readFile  func(name string) ([]byte, error)
	writeFile func(name string, data []byte) error
	log       func(...any)
}

// NewMarkDown creates a new MarkDown instance with the root directory.
// Destination (output directory) and input must be set via methods.
func NewMarkDown(rootDir, destination string, writerFile func(name string, data []byte) error) *MarkDown {
	return &MarkDown{
		rootDir:     rootDir,
		destination: destination,
		readFile:    func(name string) ([]byte, error) { return nil, errors.New("not configure reader func") },
		writeFile:   writerFile,
		log:         func(...any) {},
	}
}

// SetLog sets a custom logger function
func (m *MarkDown) SetLog(fn func(...any)) {
	if fn != nil {
		m.log = fn
	}
}

// InputPath sets the input as a file path (relative to rootDir)
func (m *MarkDown) InputPath(pathFile string, readerFile func(name string) ([]byte, error)) *MarkDown {
	m.inputPath = pathFile
	m.readFile = readerFile
	return m
}

// InputByte sets the input as a byte slice (markdown content)
func (m *MarkDown) InputByte(content []byte) *MarkDown {
	// clear other inputs
	m.readFile = func(name string) ([]byte, error) {
		return content, nil
	}

	return m
}

// InputEmbed sets the input as any ReaderFile implementation and a relative path inside it
func (m *MarkDown) InputEmbed(path string, readerFile func(name string) ([]byte, error)) *MarkDown {
	m.readFile = readerFile
	// clear other inputs
	m.inputPath = path
	return m
}

// writeIfDifferent writes data to filename only if content is different
func (m *MarkDown) writeIfDifferent(filename, content string) error {
	// Try to read existing file
	existing, err := m.readFile(filename)
	if err == nil && string(existing) == content {
		return nil // Content is the same
	}

	// Need to write
	return m.writeFile(filename, []byte(content))
}
