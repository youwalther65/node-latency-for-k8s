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
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/awslabs/node-latency-for-k8s/pkg/journal"
)

// JournalReader is a base Source helper that can Read the systemd journal
type JournalReader struct {
	Path            string
	Glob            bool
	TimestampRegex  *regexp.Regexp
	TimestampLayout string
	file            []byte
}

// ClearCache cleas the cached log
func (l *JournalReader) ClearCache() {
	l.file = nil
}

// Read will open and read all the bytes of a journal file into byte slice and then cache it
// Any further calls to Read() will use the cached byte slice.
// If the file is being updated and you need the updated contents,
// you'll need to instantiate a new JournalReader and call Read() again
// or use ClearCache()
func (l *JournalReader) Read() ([]byte, error) {
	if l.file != nil {
		return l.file, nil
	}
	resolvedPath := l.Path
	if l.Glob {
		resolvedPath = resolveNewestLogFile(l.Path)
	}
	reader, err := journal.Read(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open journal at %s: %w", resolvedPath, err)
	}
	fileBytes, err := io.ReadAll(reader)
	if err != nil {
		return fileBytes, fmt.Errorf("unable to read journal at %s: %w", resolvedPath, err)
	}
	l.file = fileBytes
	return fileBytes, nil
}

// Find searches for the passed in regexp from the log references in the JournalReader
func (l *JournalReader) Find(re *regexp.Regexp) ([]string, error) {
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
func (l *JournalReader) ParseTimestamp(line string) (time.Time, error) {
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
