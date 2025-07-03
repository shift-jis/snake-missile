package utilities

import (
	"os"

	"github.com/kpango/fastime"
	"go.uber.org/zap"
)

func MustProductionLogger() *zap.Logger {
	productionLogger, err := NewProductionLogger()
	if err != nil {
		panic(err)
	}
	return productionLogger
}

func NewProductionLogger() (*zap.Logger, error) {
	if err := os.MkdirAll("./logs", os.ModePerm); err != nil {
		return nil, err
	}

	productionConfig := zap.NewProductionConfig()
	//productionConfig.DisableCaller = true

	fileName := fastime.Now().Format("2006-0102-15.04.05") + ".log"
	productionConfig.OutputPaths = []string{"./logs/" + fileName}

	return productionConfig.Build()
}

func MustDevelopmentLogger() *zap.Logger {
	developmentLogger, err := NewDevelopmentLogger()
	if err != nil {
		panic(err)
	}
	return developmentLogger
}

func NewDevelopmentLogger() (*zap.Logger, error) {
	if err := os.MkdirAll("./logs", os.ModePerm); err != nil {
		return nil, err
	}

	developmentConfig := zap.NewDevelopmentConfig()
	//developmentConfig.DisableCaller = true

	fileName := fastime.Now().Format("2006-0102-15.04.05") + ".log"
	developmentConfig.OutputPaths = []string{"./logs/" + fileName}

	return developmentConfig.Build()
}
