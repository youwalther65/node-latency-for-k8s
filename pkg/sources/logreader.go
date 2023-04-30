/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sources

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// LogReader is a base Source helper that can Read file contents, cache, and support Glob file paths
// Other Sources can be built on-top of the LogSrc
type LogReader struct {
	Path            string
	Glob            bool
	TimestampRegex  *regexp.Regexp
	TimestampLayout string
	file            []byte
}

// ClearCache cleas the cached log
func (l *LogReader) ClearCache() {
	l.file = nil
}

// Read will open and read all the bytes of a log file into byte slice and then cache it
// Any further calls to Read() will use the cached byte slice.
// If the file is being updated and you need the updated contents,
// you'll need to instantiate a new LogSrc and call Read() again
func (l *LogReader) Read() ([]byte, error) {
	if l.file != nil {
		return l.file, nil
	}
	resolvedPath := l.Path
	if l.Glob {
		resolvedPath = resolveOldestLogFile(l.Path)
	}
	var reader io.Reader
	file, err := os.Open(resolvedPath)
	// If there's a problem opening the log file, use the journal instead
	if err != nil {
		return nil, fmt.Errorf("unable to open log file %s: %w", resolvedPath, err)
	}
	defer file.Close()
	if strings.HasSuffix(resolvedPath, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("unable to create gzip reader for file %s: %w", file.Name(), err)
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = bufio.NewReader(file)
	}
	fileBytes, err := io.ReadAll(reader)
	if err != nil {
		return fileBytes, fmt.Errorf("unable to read file %s: %w", file.Name(), err)
	}
	l.file = fileBytes
	return fileBytes, nil
}

// Find searches for the passed in regexp from the log references in the LogReader
func (l *LogReader) Find(re *regexp.Regexp) ([]string, error) {
	// Read the log file
	messages, err := l.Read()
	if err != nil {
		return nil, err
	}
	// Find all occurrences of the regex in the log file
	lines := re.FindAll(messages, -1)
	if len(lines) == 0 {
		return nil, fmt.Errorf("no matches in %s for regex \"%s\"", l.Path, re.String())
	}
	var lineStrs []string
	for _, line := range lines {
		lineStrs = append(lineStrs, string(line))
	}
	return lineStrs, nil
}

// ParseTimestamp usese the configured timestamp regex to find a timestamp from the passed in log line and return as a time.Time
func (l *LogReader) ParseTimestamp(line string) (time.Time, error) {
	rawTS := l.TimestampRegex.FindString(line)
	if rawTS == "" {
		return time.Time{}, fmt.Errorf("unable to find timestamp on log line matching regex: \"%s\" \"%s\"", l.TimestampRegex.String(), line)
	}
	rawTS = spaceRE.ReplaceAllString(rawTS, " ")

	suffix := ""
	// Convert timestamp to a time.Time type
	if !strings.Contains(rawTS, fmt.Sprint(time.Now().Year())) {
		suffix = fmt.Sprintf(" %d", time.Now().Year())
	}
	ts, err := time.Parse(l.TimestampLayout, fmt.Sprintf("%s%s", rawTS, suffix))
	if err != nil {
		return time.Time{}, err
	}
	return ts, nil
}
