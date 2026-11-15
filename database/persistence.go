// database/persistence.go
package database

import (
	"os"
)

// AOFLogger handles writing commands to the Append-Only File.
type AOFLogger struct {
	file *os.File
}

// NewAOFLogger opens (or creates) the AOF file in append mode.
func NewAOFLogger(filename string) (*AOFLogger, error) {
	// os.O_CREATE: Create file if it doesn't exist.
	// os.O_RDWR: Open for reading and writing.
	// os.O_APPEND: Append data to the end of the file.
	// 0644: Standard file permissions (read/write for owner, read for others).
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &AOFLogger{file: file}, nil
}

// WriteCommand appends a single command string to the AOF file.
func (a *AOFLogger) WriteCommand(cmd string) error {
	// Write the command followed by a newline character.
	_, err := a.file.WriteString(cmd + "\n")
	return err
}

// Close safely closes the AOF file.
func (a *AOFLogger) Close() error {
	return a.file.Close()
}