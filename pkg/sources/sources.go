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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var (
	spaceRE = regexp.MustCompile(`\s+`)
)

// Source is an interface representing a source of events which have a time stamp or latency associated with them.
// Most often source is a log file or an API.
type Source interface {
	// Find finds the string in the source using a source specific method (could be regex or HTTP path)
	// If no time.Time could be found an error is returned
	Find(event *Event) ([]FindResult, error)
	// Name is the source name identifier
	Name() string
	// ClearCache clears any cached source data
	ClearCache()
	// String is a human friendly version of the source, usually the log filepath
	String() string
}

// FindResult is all data associated with a find including the raw Line data
type FindResult struct {
	Line      string
	Timestamp time.Time
	Comment   string
	Err       error
}

type FindFunc func(s Source, log []byte) ([]string, error)
type CommentFunc func(matchedLine string) string

// Event defines what is being timed from a specific source
type Event struct {
	Name          string      `json:"name"`
	Metric        string      `json:"metric"`
	MatchSelector string      `json:"matchSelector"`
	Terminal      bool        `json:"terminal"`
	SrcName       string      `json:"src"`
	Src           Source      `json:"-"`
	CommentFn     CommentFunc `json:"-"`
	FindFn        FindFunc    `json:"-"`
}

// Match Selector consts for an Event's MatchSelector
const (
	EventMatchSelectorFirst = "first"
	EventMatchSelectorLast  = "last"
	EventMatchSelectorAll   = "all"
)

// Timing is a specific instance of an Event timing
type Timing struct {
	Event     *Event        `json:"event"`
	Timestamp time.Time     `json:"timestamp"`
	T         time.Duration `json:"seconds"`
	Comment   string        `json:"comment"`
	Error     error         `json:"error"`
}

// SelectMaches will filter raw results based on the provided matchSelector
func SelectMatches(results []FindResult, matchSelector string) []FindResult {
	if len(results) == 0 {
		return nil
	}
	switch matchSelector {
	case EventMatchSelectorFirst:
		return []FindResult{results[0]}
	case EventMatchSelectorLast:
		return []FindResult{results[len(results)-1]}
	case EventMatchSelectorAll:
		return results
	}
	return results
}

// CommentMatchedLine is a helper func that returns a func that can be used as a CommentFunc in an Event
// The func will use the matched line as the comment
func CommentMatchedLine() func(matchedLine string) string {
	return func(matchedLine string) string {
		return matchedLine
	}
}

func resolveOldestLogFile(globPath string) string {
	logFiles := sortedAscLogFiles(globPath)
	if len(logFiles) == 0 {
		return ""
	}
	return logFiles[0]
}

func resolveNewestLogFile(globPath string) string {
	logFiles := sortedAscLogFiles(globPath)
	if len(logFiles) == 0 {
		return ""
	}
	return logFiles[len(logFiles)-1]
}

func sortedAscLogFiles(globPath string) []string {
	matches, err := filepath.Glob(globPath)
	if err != nil || len(matches) == 0 {
		return nil
	}
	// sort to find the oldest file for initial startup timings if the logs were rotated
	sort.Slice(matches, func(i, j int) bool {
		iFile, err := os.Open(matches[i])
		if err != nil {
			return matches[i] < matches[j]
		}
		defer iFile.Close()
		jFile, err := os.Open(matches[j])
		if err != nil {
			return matches[i] < matches[j]
		}
		defer jFile.Close()
		iStat, err := iFile.Stat()
		if err != nil {
			return matches[i] < matches[j]
		}
		jStat, err := jFile.Stat()
		if err != nil {
			return matches[i] < matches[j]
		}
		return iStat.ModTime().Unix() < jStat.ModTime().Unix()
	})
	return matches
}
