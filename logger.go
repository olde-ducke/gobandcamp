package main

import (
	"os"
	"time"
)

type debugLogger struct {
	logFile *os.File
}

func (logger *debugLogger) initialize() error {
	file, err := os.Create("dump.log")
	if err != nil {
		return err
	}
	logger.logFile = file
	return nil
}

func (logger *debugLogger) finalize() error {
	return logger.logFile.Close()
}

// FIXME: no error handling at all, shouldn't be a problem though?
func (logger *debugLogger) writeToLogFile(str string) {
	if logger.logFile != nil {
		logger.logFile.WriteString(time.Now().Format(time.ANSIC) + str + "\n")
	}
}
