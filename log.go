package main

import (
	"os"
	"io"
	"log"
)

// Open the log file for logging
func openLog() error {
	logFile, err := os.OpenFile(gConfig.Log, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	out := io.MultiWriter(logFile, os.Stderr)
	log.SetOutput(out)
	return nil
}

// Open the log, or fail immediately
func requireLog() {
	err := openLog()
	if err != nil {
		log.Printf("LOG_ERROR Error=%q\n", err.Error())
		os.Exit(0)
	}
}