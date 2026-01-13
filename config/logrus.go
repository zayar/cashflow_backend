package config

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	logg *logrus.Logger
)

func GetLogger() *logrus.Logger {
	return logg
}

func init() {
	logg = logrus.New()
	logg.SetFormatter(&logrus.JSONFormatter{})
	logg.SetLevel(logrus.ErrorLevel)
	logg.SetOutput(os.Stdout)

	// You could set this to any `io.Writer` such as a file
	// file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err == nil {
	// 	logg.Out = file
	// } else {
	// 	logg.Info("Failed to log to file, using default stderr")
	// }
}

func LogError(logger *logrus.Logger, moduleName string, funcName string, context string, data any, err error) {
	if data != nil {
		logger.WithFields(logrus.Fields{
			"module":   moduleName,
			"funcName": funcName,
			"context":  context,
			"data":     data,
		}).Error(err.Error())
	} else {
		logger.WithFields(logrus.Fields{
			"module":   moduleName,
			"funcName": funcName,
			"context":  context,
		}).Error(err.Error())
	}
}
