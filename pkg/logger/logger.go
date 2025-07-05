package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

// Fields is an alias for logrus.Fields
type Fields = logrus.Fields

// InitLogger initializes the global logger with specified log level
func InitLogger(level string) {
	Logger = logrus.New()
	
	// Set log level
	logLevel := parseLogLevel(level)
	Logger.SetLevel(logLevel)
	
	// Set formatter
	Logger.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	
	// Set output
	Logger.SetOutput(os.Stdout)
	
	Logger.WithFields(logrus.Fields{
		"level": logLevel.String(),
	}).Info("Logger initialized")
}

// parseLogLevel parses string log level to logrus.Level
func parseLogLevel(level string) logrus.Level {
	switch strings.ToLower(level) {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}

// GetLogger returns the global logger instance
func GetLogger() *logrus.Logger {
	if Logger == nil {
		InitLogger("info")
	}
	return Logger
}