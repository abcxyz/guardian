// Copyright 2024 The Authors (see AUTHORS file)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package modifiers provides the functionality to parse meta modifiers from text.
package modifiers

import (
	"context"
	"regexp"
)

const (
	MetaKeyGuardianDestroy = "GUARDIAN_DESTROY"
	MetaValueAll           = "all"
)

var (
	// newline is a regexp to split strings at line breaks.
	newline = regexp.MustCompile("\r?\n")

	// metaValuesKVRegex is a regex to split key value pairs for comment modifiers.
	metaValuesKVRegex = regexp.MustCompile(`^(GUARDIAN_[A-Z0-9_]+)=(.*)$`)
)

// MetaValues is a map of key value pairs to store meta values.
type MetaValues map[string][]string

// ParseBodyMetaValues parses the modifiders from a contents string.
func ParseBodyMetaValues(ctx context.Context, contents string) MetaValues {
	metaValues := make(MetaValues, 0)

	for _, line := range newline.Split(contents, -1) {
		match := metaValuesKVRegex.FindStringSubmatch(line)
		if len(match) == 3 {
			metaKey := match[1]
			metaValue := match[2]
			metaValues[metaKey] = append(metaValues[metaKey], metaValue)
		}
	}
	return metaValues
}
