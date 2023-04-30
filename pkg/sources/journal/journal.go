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

// Package journal is a latency timing source for /var/log/journal/*
package journal

import (
	"regexp"
	"sort"

	"github.com/awslabs/node-latency-for-k8s/pkg/sources"
)

var (
	Name            = "Journal"
	DefaultPath     = "/node/var/log/journal/*"
	TimestampFormat = regexp.MustCompile(`[0-9]{4}\-[0-9]{2}\-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\+[0-9]{4}`)
	TimestampLayout = "2006-01-02T15:04:05-0700"
)

// Source is the /var/log/journal log source
type Source struct {
	journalReader *sources.JournalReader
}

// New instantiates a new instance of a journal source
func New(path string) *Source {
	return &Source{
		journalReader: &sources.JournalReader{
			Path:            path,
			Glob:            true,
			TimestampRegex:  TimestampFormat,
			TimestampLayout: TimestampLayout,
		},
	}
}

// ClearCache will clear the journal reader cache
func (s Source) ClearCache() {
	s.journalReader.ClearCache()
}

// String is a human readable string of the source, usually the journal file path or directory
func (s Source) String() string {
	return s.journalReader.Path
}

// Name is the name of the source
func (s Source) Name() string {
	return Name
}

// FindByRegex is a helper func that returns a FindFunc to search for a regex in a log source that can be used in an Event
func (s Source) FindByRegex(re *regexp.Regexp) sources.FindFunc {
	return func(_ sources.Source, log []byte) ([]string, error) {
		return s.journalReader.Find(re)
	}
}

// Find will use the Event's FindFunc and CommentFunc to search the log source and return the results based on the Event's matcher
func (s Source) Find(event *sources.Event) ([]sources.FindResult, error) {
	logBytes, err := s.journalReader.Read()
	if err != nil {
		return nil, err
	}
	matchedLines, err := event.FindFn(s, logBytes)
	if err != nil {
		return nil, err
	}
	var results []sources.FindResult
	for _, line := range matchedLines {
		ts, err := s.journalReader.ParseTimestamp(line)
		comment := ""
		if event.CommentFn != nil {
			comment = event.CommentFn(line)
		}
		results = append(results, sources.FindResult{
			Line:      line,
			Timestamp: ts,
			Err:       err,
			Comment:   comment,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.UnixMicro() < results[j].Timestamp.UnixMicro()
	})
	return sources.SelectMatches(results, event.MatchSelector), nil
}
