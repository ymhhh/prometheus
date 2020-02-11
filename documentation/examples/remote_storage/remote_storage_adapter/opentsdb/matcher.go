// Copyright 2013 The JD Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opentsdb

import (
	"regexp"
)

type seriesMatcher []*LabelMatcher

func (s seriesMatcher) Match(tags map[string]TagValue) bool {
	if tags == nil {
		tags = make(map[string]TagValue)
	}
	for _, m := range s {
		if !m.Match(string(tags[m.Name])) {
			return false
		}
	}
	return true
}

// NewLabelMatcher returns a LabelMatcher object ready to use.
func NewLabelMatcher(matchType MatchType, name string, value string) (*LabelMatcher, error) {
	m := &LabelMatcher{
		Type:  matchType,
		Name:  name,
		Value: value,
	}
	if matchType == RegexMatch || matchType == RegexNoMatch {
		re, err := regexp.Compile("^(?:" + string(value) + ")$")
		if err != nil {
			return nil, err
		}
		m.re = re
	}
	return m, nil
}

// LabelMatcher models the matching of a label. Create with NewLabelMatcher.
type LabelMatcher struct {
	Type  MatchType
	Name  string
	Value string
	re    *regexp.Regexp
}

// MatchType is an enum for label matching types.
type MatchType int

// Possible MatchTypes.
const (
	Equal MatchType = iota
	NotEqual
	RegexMatch
	RegexNoMatch
)

func (m MatchType) String() string {
	typeToStr := map[MatchType]string{
		Equal:        "=",
		NotEqual:     "!=",
		RegexMatch:   "=~",
		RegexNoMatch: "!~",
	}
	if str, ok := typeToStr[m]; ok {
		return str
	}
	panic("unknown match type")
}

// Match returns true if the label matcher matches the supplied label value.
func (m *LabelMatcher) Match(v string) bool {
	switch m.Type {
	case Equal:
		return m.Value == v
	case NotEqual:
		return m.Value != v
	case RegexMatch:
		return m.re.MatchString(string(v))
	case RegexNoMatch:
		return !m.re.MatchString(string(v))
	default:
		panic("invalid match type")
	}
}
