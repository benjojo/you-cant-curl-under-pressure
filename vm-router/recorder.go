package main

import (
	"encoding/binary"
	"io"
	"strings"
	"time"
)

// Returns true if the last array emitted was "TEST_PASSED"
func inlineTTYRecRecorder(input io.Reader, transparentOutput io.Writer, recordingFileOutput io.Writer) bool {
	lastBuf := make([]byte, 1)
	dataCount := 0
	buf := make([]byte, 1024)

	for {
		buf = make([]byte, 1024)
		n, err := input.Read(buf)
		if err != nil {
			if strings.Contains(string(lastBuf), "TEST_PASSED") {
				return true
			}
			return false
		}
		dataCount += n

		transparentOutput.Write(buf[:n])
		ts := time.Now()

		unixmilli := uint32(ts.Unix() - (ts.UnixNano() / 1000000))
		unixsecs := uint32(ts.Unix())

		binary.Write(recordingFileOutput, binary.LittleEndian, unixsecs)
		binary.Write(recordingFileOutput, binary.LittleEndian, unixmilli)
		binary.Write(recordingFileOutput, binary.LittleEndian, uint32(n))
		recordingFileOutput.Write(buf[:n])
		lastBuf = buf[:n]
		if dataCount > 100000 {
			transparentOutput.Write([]byte("\r\n\n\nToo much data written to terminal."))
			return false
		}
	}
}

func oneTimeWriteTottyFile(input []byte, recordingFileOutput io.Writer) {
	ts := time.Now()

	unixmilli := uint32(ts.Unix() - (ts.UnixNano() / 1000000))
	unixsecs := uint32(ts.Unix())

	binary.Write(recordingFileOutput, binary.LittleEndian, unixsecs)
	binary.Write(recordingFileOutput, binary.LittleEndian, unixmilli)
	binary.Write(recordingFileOutput, binary.LittleEndian, uint32(len(input)))
	recordingFileOutput.Write(input)
}
