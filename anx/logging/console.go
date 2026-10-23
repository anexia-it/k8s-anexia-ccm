// Package logging adds application-specific logging formats to the Kubernetes logging stack.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/component-base/featuregate"
	logsapi "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2/textlogger"
)

const ConsoleFormat = "console"

var timeNow = time.Now

func init() {
	if err := logsapi.RegisterLogFormat(ConsoleFormat, consoleFactory{}, logsapi.LoggingStableOptions); err != nil {
		panic(fmt.Sprintf("registering %q log format: %v", ConsoleFormat, err))
	}
}

type consoleFactory struct{}

func (consoleFactory) Feature() featuregate.Feature {
	return logsapi.LoggingStableOptions
}

func (consoleFactory) Create(config logsapi.LoggingConfiguration, options logsapi.LoggingOptions) (logr.Logger, logsapi.RuntimeControl) {
	state := &consoleState{
		output: options.ErrorStream,
	}
	state.verbosity.Store(uint32(config.Verbosity))

	logger := logr.New(&consoleSink{state: state})
	return logger, logsapi.RuntimeControl{
		SetVerbosityLevel: func(verbosity uint32) error {
			state.verbosity.Store(verbosity)
			return nil
		},
	}
}

type consoleState struct {
	mutex     sync.Mutex
	output    io.Writer
	verbosity atomic.Uint32
}

type consoleSink struct {
	state  *consoleState
	name   string
	values []any
}

var (
	_ logr.LogSink                = (*consoleSink)(nil)
	_ textlogger.KlogBufferWriter = (*consoleSink)(nil)
)

func (sink *consoleSink) Init(logr.RuntimeInfo) {}

func (sink *consoleSink) Enabled(level int) bool {
	return level >= 0 && uint32(level) <= sink.state.verbosity.Load()
}

func (sink *consoleSink) Info(level int, message string, keysAndValues ...any) {
	logLevel := "INFO"
	if level > 0 {
		logLevel = "DEBUG"
		keysAndValues = append([]any{"v", level}, keysAndValues...)
	}

	sink.write(logLevel, message, nil, keysAndValues)
}

func (sink *consoleSink) Error(err error, message string, keysAndValues ...any) {
	sink.write("ERROR", message, err, keysAndValues)
}

func (sink *consoleSink) WithValues(keysAndValues ...any) logr.LogSink {
	clone := *sink
	clone.values = append(append([]any(nil), sink.values...), keysAndValues...)
	return &clone
}

func (sink *consoleSink) WithName(name string) logr.LogSink {
	clone := *sink
	if clone.name == "" {
		clone.name = name
	} else {
		clone.name += "/" + name
	}
	return &clone
}

// WriteKlogBuffer receives legacy klog messages with their original severity intact.
func (sink *consoleSink) WriteKlogBuffer(buffer []byte) {
	level, message := parseKlogBuffer(buffer)
	sink.write(level, message, nil, nil)
}

func (sink *consoleSink) write(level, message string, err error, keysAndValues []any) {
	metadata := make([]any, 0, len(sink.values)+len(keysAndValues)+4)
	if sink.name != "" {
		metadata = append(metadata, "logger", sink.name)
	}
	metadata = append(metadata, sink.values...)
	if err != nil {
		metadata = append(metadata, "error", err)
	}
	metadata = append(metadata, keysAndValues...)

	line := fmt.Sprintf(
		"%s  %-5s  %s",
		timeNow().Format("2006-01-02T15:04:05.000Z07:00"),
		level,
		message,
	)
	if formattedMetadata := formatMetadata(metadata); formattedMetadata != "" {
		line += "  | " + formattedMetadata
	}
	line += "\n"

	sink.state.mutex.Lock()
	defer sink.state.mutex.Unlock()
	_, _ = io.WriteString(sink.state.output, line)
}

func parseKlogBuffer(buffer []byte) (level, message string) {
	line := strings.TrimSuffix(string(buffer), "\n")
	level = "INFO"
	if len(line) > 0 {
		switch line[0] {
		case 'W':
			level = "WARN"
		case 'E', 'F':
			level = "ERROR"
		}
	}

	headerEnd := strings.Index(line, "]")
	if headerEnd < 0 {
		return level, line
	}

	message = strings.TrimSpace(line[headerEnd+1:])
	return level, message
}

func formatMetadata(keysAndValues []any) string {
	fields := make([]string, 0, (len(keysAndValues)+1)/2)
	for index := 0; index < len(keysAndValues); index += 2 {
		key := fmt.Sprint(keysAndValues[index])
		value := any("<missing>")
		if index+1 < len(keysAndValues) {
			value = keysAndValues[index+1]
		}

		if marshaler, ok := value.(logr.Marshaler); ok {
			value = marshaler.MarshalLog()
		}
		fields = append(fields, key+"="+formatValue(value))
	}
	return strings.Join(fields, " ")
}

func formatValue(value any) string {
	if err, ok := value.(error); ok {
		return strconv.Quote(err.Error())
	}

	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return strconv.Quote(fmt.Sprint(value))
}
