package logger

type (
	Logger interface {
		Error(err error, message string, params ...any)
		Info(message string, params ...any)
		Warning(message string, params ...any)
	}
)

var (
	_loggers []Logger
)

func init() {
	_loggers = make([]Logger, 0)
}

func AddLogger(logger Logger) {
	_loggers = append(_loggers, logger)
}

func Info(message string, params ...any) {
	for _, logger := range _loggers {
		logger.Info(message, params...)
	}
}

func Warning(message string, params ...any) {
	for _, logger := range _loggers {
		logger.Warning(message, params...)
	}
}

func Error(err error, message string, params ...any) {
	for _, logger := range _loggers {
		logger.Error(err, message, params...)
	}
}
