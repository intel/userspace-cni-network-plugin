// Copyright 2020 Intel Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const panicRegexp = `(?s)` +
	`\[panic\] Logging: Error occurred!\n.*` +
	`\[panic\] ========= Stack trace output ========\n.*` +
	`\[panic\] Userspace CNI Panic\n.*` +
	`\[panic\] ========= Stack trace output end ========\n$`

func TestString(t *testing.T) {
	testCases := []struct {
		name      string
		level     Level
		expResult string
	}{
		{
			name:      "level panic",
			level:     PanicLevel,
			expResult: "panic",
		},
		{
			name:      "level error",
			level:     ErrorLevel,
			expResult: "error",
		},
		{
			name:      "level warning",
			level:     WarningLevel,
			expResult: "warning",
		},
		{
			name:      "level info",
			level:     InfoLevel,
			expResult: "info",
		},
		{
			name:      "level debug",
			level:     DebugLevel,
			expResult: "debug",
		},
		{
			name:      "level verbose",
			level:     VerboseLevel,
			expResult: "verbose",
		},
		{
			name:      "level unknown",
			level:     UnknownLevel,
			expResult: "unknown",
		},
		{
			name:      "max level",
			level:     MaxLevel,
			expResult: "unknown",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.level.String()
			assert.Equal(t, tc.expResult, result, "Unexpected result")

		})
	}
}

func TestLogFunctions(t *testing.T) {
	testCases := []struct {
		name          string
		logFunc       string
		level         Level
		format        string
		arguments     []interface{}
		logLevel      Level
		expResultStdE string
		expResultFile string
	}{
		{
			name:          "log to file, stderr, level: panic, args: string",
			logFunc:       "Printf",
			level:         PanicLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"string"},
			expResultStdE: ` \[panic\] Logging: string\n$`,
			expResultFile: ` \[panic\] Logging: string\n$`,
		},
		{
			name:          "log to file, stderr, level: panic, args: string, int",
			logFunc:       "Printf",
			level:         PanicLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v %v",
			arguments:     []interface{}{"string", 42},
			expResultStdE: ` \[panic\] Logging: string 42\n$`,
			expResultFile: ` \[panic\] Logging: string 42\n$`,
		},
		{
			name:          "log to file, stderr, level: panic, args: error",
			logFunc:       "Printf",
			level:         PanicLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[panic\] Logging: Error occurred!\n$`,
			expResultFile: ` \[panic\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress printf verbose log entry due to low log level",
			logFunc:       "Printf",
			level:         VerboseLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file only, level: panic, args: string",
			logFunc:       "Printf",
			level:         PanicLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"string"},
			expResultStdE: "",
			expResultFile: ` \[panic\] Logging: string\n$`,
		},
		{
			name:          "log to stderr only, level: panic, args: string",
			logFunc:       "Printf",
			level:         PanicLevel,
			logLevel:      InfoLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"string"},
			expResultStdE: ` \[panic\] Logging: string\n$`,
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: verbose, args: error",
			logFunc:       "Verbosef",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[verbose\] Logging: Error occurred!\n$`,
			expResultFile: ` \[verbose\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress verbose log entry due to low log level",
			logFunc:       "Verbosef",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: debug, args: error",
			logFunc:       "Debugf",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[debug\] Logging: Error occurred!\n$`,
			expResultFile: ` \[debug\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress debug log entry due to low log level",
			logFunc:       "Debugf",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: info, args: error",
			logFunc:       "Infof",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[info\] Logging: Error occurred!\n$`,
			expResultFile: ` \[info\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress info log entry due to low log level",
			logFunc:       "Infof",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: warning, args: error",
			logFunc:       "Warningf",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[warning\] Logging: Error occurred!\n$`,
			expResultFile: ` \[warning\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress warning log entry due to low log level",
			logFunc:       "Warningf",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: error, args: error",
			logFunc:       "Errorf",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: ` \[error\] Logging: Error occurred!\n$`,
			expResultFile: ` \[error\] Logging: Error occurred!\n$`,
		},
		{
			name:          "suppress error log entry due to low log level",
			logFunc:       "Errorf",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{"arg"},
			expResultStdE: "",
			expResultFile: "",
		},
		{
			name:          "log to file, stderr, level: panic, args: error, logLevel: MaxLevel",
			logFunc:       "Panicf",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: panicRegexp,
			expResultFile: panicRegexp,
		},
		{
			name:          "log to file, stderr, level: panic, args: error, logLevel: UnknownLevel",
			logFunc:       "Panicf",
			logLevel:      MaxLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: panicRegexp,
			expResultFile: panicRegexp,
		},
		{
			name:          "log to file, stderr, level: panic, args: error, logLevel: PanicLevel",
			logFunc:       "Panicf",
			logLevel:      PanicLevel,
			format:        "Logging: %v",
			arguments:     []interface{}{errors.New("Error occurred!")},
			expResultStdE: panicRegexp,
			expResultFile: panicRegexp,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			origLevel := loggingLevel
			origStderr := loggingStderr
			origFp := loggingFp
			defer func() {
				loggingLevel = origLevel
				loggingStderr = origStderr
				loggingFp = origFp
			}()

			loggingLevel = tc.logLevel
			loggingStderr = false
			loggingFp = nil
			if tc.expResultStdE != "" {
				loggingStderr = true
			}

			// capture messages from Printf written to stderr
			var err error
			stdR, stdW, err := os.Pipe()
			require.NoError(t, err, "Can't capture stderr")
			origStdErr := os.Stderr
			os.Stderr = stdW

			// capture messages from Printf written to file if needed
			var logR, logW *os.File
			if tc.expResultFile != "" {
				logR, logW, err = os.Pipe()
				require.NoError(t, err, "Can't capture log file")
				loggingFp = logW
			}

			switch tc.logFunc {
			case "Printf":
				Printf(tc.level, tc.format, tc.arguments...)
			case "Verbosef":
				Verbosef(tc.format, tc.arguments...)
			case "Debugf":
				Debugf(tc.format, tc.arguments...)
			case "Infof":
				Infof(tc.format, tc.arguments...)
			case "Warningf":
				Warningf(tc.format, tc.arguments...)
			case "Errorf":
				_ = Errorf(tc.format, tc.arguments...)
			case "Panicf":
				Panicf(tc.format, tc.arguments...)
			default:
				t.Fatalf("Unknown function type %q", tc.logFunc)
			}

			os.Stderr = origStdErr
			stdW.Close()
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, stdR)

			if tc.expResultStdE != "" {
				assert.Regexp(t, tc.expResultStdE, buf.String(), "Unexpected stderr log")
			} else {
				assert.Empty(t, buf.String(), "Unexpected stderr log")
			}

			if tc.expResultFile != "" {
				logW.Close()
				_, _ = io.Copy(&buf, logR)
				assert.Regexp(t, tc.expResultFile, buf.String(), "Unexpected file log")
			}
		})
	}
}

func TestGetLoggingLevel(t *testing.T) {
	testCases := []struct {
		name      string
		level     string
		expResult Level
	}{
		{
			name:      "level panic",
			level:     "panic",
			expResult: PanicLevel,
		},
		{
			name:      "level PANIC",
			level:     "PANIC",
			expResult: PanicLevel,
		},
		{
			name:      "level PaNiC",
			level:     "PaNiC",
			expResult: PanicLevel,
		},
		{
			name:      "level error",
			level:     "error",
			expResult: ErrorLevel,
		},
		{
			name:      "level warning",
			level:     "warning",
			expResult: WarningLevel,
		},
		{
			name:      "level info",
			level:     "info",
			expResult: InfoLevel,
		},
		{
			name:      "level debug",
			level:     "debug",
			expResult: DebugLevel,
		},
		{
			name:      "level verbose",
			level:     "verbose",
			expResult: VerboseLevel,
		},
		{
			name:      "level unknown",
			level:     "unknown",
			expResult: UnknownLevel,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetLoggingLevel(tc.level)
			assert.Equal(t, tc.expResult, result, "Unexpected result")

		})
	}
}

func TestSetLogLevel(t *testing.T) {
	testCases := []struct {
		name      string
		level     string
		origLevel Level
		expResult Level
	}{
		{
			name:      "change level from info to verbose",
			level:     "verbose",
			origLevel: InfoLevel,
			expResult: VerboseLevel,
		},
		{
			name:      "change level from info to vErBoSe",
			level:     "vErBoSe",
			origLevel: InfoLevel,
			expResult: VerboseLevel,
		},
		{
			name:      "change level from verbose to panic",
			level:     "panic",
			origLevel: VerboseLevel,
			expResult: PanicLevel,
		},
		{
			name:      "change level from warning to warning",
			level:     "warning",
			origLevel: WarningLevel,
			expResult: WarningLevel,
		},
		{
			name:      "ignore level change to unknown level",
			level:     "unknown",
			origLevel: InfoLevel,
			expResult: InfoLevel,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			origLevel := loggingLevel
			defer func() {
				loggingLevel = origLevel
			}()
			loggingLevel = tc.origLevel
			SetLogLevel(tc.level)
			assert.Equal(t, tc.expResult, loggingLevel, "Unexpected result")
		})
	}
}

func TestSetLogStderr(t *testing.T) {
	origStderr := loggingStderr

	SetLogStderr(true)
	assert.True(t, loggingStderr, "Unexpected result")

	SetLogStderr(false)
	assert.False(t, loggingStderr, "Unexpected result")

	SetLogStderr(false)
	assert.False(t, loggingStderr, "Unexpected result")

	SetLogStderr(true)
	assert.True(t, loggingStderr, "Unexpected result")

	loggingStderr = origStderr
}

func TestSetLogFile(t *testing.T) {
	testCases := []struct {
		name     string
		file     string
		expError string
	}{
		{
			name:     "open log file",
			file:     "#random#",
			expError: "",
		},
		{
			name:     "fail to open log file in read only directory",
			file:     "/proc/test-logging.log",
			expError: "Userspace-CNI logging: cannot open",
		},
		{
			name:     "fail to open log file with empty name",
			file:     "",
			expError: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			origFp := loggingFp
			defer func() {
				loggingFp = origFp
			}()

			if tc.file == "#random#" {
				logFile, err := os.CreateTemp("/tmp", "test-logging-")
				require.NoError(t, err, "Can't create log file")
				logFile.Close()
				os.Remove(logFile.Name())
				tc.file = logFile.Name()
			}

			// capture error messages from SetLogFile written to stderr
			stdR, stdW, err := os.Pipe()
			require.NoError(t, err, "Can't capture stderr")
			origStdErr := os.Stderr
			os.Stderr = stdW

			SetLogFile(tc.file)

			os.Stderr = origStdErr
			stdW.Close()
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, stdR)

			if tc.expError == "" {
				assert.Empty(t, buf.String(), "Unexpected error")
				if tc.file != "" {
					assert.Equal(t, tc.file, loggingFp.Name(), "Unexpected error")
					require.NotNil(t, loggingFp, "Logging file shall be opened.")
					loggingFp.Close()
					os.Remove(loggingFp.Name())
				} else {
					require.Nil(t, loggingFp, "Logging file shall not be set.")
				}
			} else {
				assert.Contains(t, buf.String(), tc.expError, "Unexpected error")
			}
		})
	}
}
