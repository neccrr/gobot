package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

var (
	logFile *os.File
)

// Initialize logger to write to both console and file
func initLogger() error {
	// Create logs directory if it doesn't exist
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		os.Mkdir("logs", 0755)
	}

	// Create log file with timestamp
	filename := fmt.Sprintf("logs/bot_%s.log", time.Now().Format("2006-01-02_15-04-05"))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logFile = file
	log.SetOutput(file)

	// Also log to console
	log.SetFlags(0) // We'll handle formatting ourselves
	log.SetOutput(io.MultiWriter(os.Stdout, file))

	return nil
}

// Structured logging function
func LogCommand(level, command, userID, username, code, guildID, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	logEntry := fmt.Sprintf("[%s] %s: command=%s user=%s(%s) guild=%s code=%s",
		timestamp, level, command, username, userID, guildID, code)

	if message != "" {
		logEntry += " message=" + message
	}

	log.Println(logEntry)
}

// Log API calls
func LogAPIRequest(code string, statusCode int, responseTime time.Duration) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	log.Printf("[%s] API: code=%s status=%d response_time=%v",
		timestamp, code, statusCode, responseTime)
}

// Close log file
func closeLogger() {
	if logFile != nil {
		logFile.Close()
	}
}