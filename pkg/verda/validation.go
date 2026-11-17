// Copyright 2026 Verda Cloud Oy
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

// validation.go contains shared validation helpers used across multiple service
// type files (e.g. container_deployments_types.go, serverless_jobs_types.go).
//
// Service-specific validation logic (Validate methods, validateCreate* functions)
// lives in the corresponding *_types.go file. Only put helpers here when they
// are reused by more than one service domain.

package verda

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// IsLatestTag checks if a container image uses the "latest" tag.
// The API does not allow "latest" tag - a specific version must be used.
func IsLatestTag(image string) bool {
	if strings.HasSuffix(image, ":latest") {
		return true
	}
	if !strings.Contains(image, ":") && !strings.Contains(image, "@") {
		return true
	}
	return false
}

// fieldErrors maps a field's JSON name (or Go name when untagged) to why it
// failed. Error lists fields sorted by name and wraps a nested fieldErrors in
// parentheses: "a: cannot be blank; b: (c: must be a valid value.)."
type fieldErrors map[string]error

func (e fieldErrors) Error() string {
	var s strings.Builder
	for i, field := range slices.Sorted(maps.Keys(e)) {
		if i > 0 {
			s.WriteString("; ")
		}
		s.WriteString(field + ": ")
		if nested, ok := e[field].(fieldErrors); ok {
			s.WriteString("(" + nested.Error() + ")")
		} else {
			s.WriteString(e[field].Error())
		}
	}
	s.WriteString(".")
	return s.String()
}

// add records err for field; a nil err leaves the field valid.
func (e fieldErrors) add(field string, err error) {
	if err != nil {
		e[field] = err
	}
}

// err returns nil when no field failed, so Validate never returns a non-nil empty map.
func (e fieldErrors) err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

var (
	errBlank        = errors.New("cannot be blank")
	errInvalidValue = errors.New("must be a valid value")
	errFormat       = errors.New("must be in a valid format")
)

// firstErr returns the first failing check; a field reports one error at a time.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// when returns err only if cond holds, for checks that apply conditionally.
func when(cond bool, err error) error {
	if cond {
		return err
	}
	return nil
}

func required[T comparable](v T) error {
	var zero T
	if v == zero {
		return errBlank
	}
	return nil
}

func requiredSlice[T any](v []T) error {
	if len(v) == 0 {
		return errBlank
	}
	return nil
}

// requiredPtr rejects a nil pointer and a pointer to the zero value.
func requiredPtr[T comparable](p *T) error {
	if p == nil {
		return errBlank
	}
	return required(*p)
}

// notEmptyIfSet accepts nil (field not sent) but rejects a pointer to "".
func notEmptyIfSet(p *string) error {
	if p != nil && *p == "" {
		return errBlank
	}
	return nil
}

// oneOf accepts the zero value (field not sent) or any of allowed.
func oneOf[T comparable](v T, allowed ...T) error {
	var zero T
	if v == zero || slices.Contains(allowed, v) {
		return nil
	}
	return errInvalidValue
}

// maxLength checks a length (bytes for strings, elements for slices) against maxLen.
func maxLength(length, maxLen int) error {
	if length > maxLen {
		return fmt.Errorf("the length must be no more than %d", maxLen)
	}
	return nil
}

func maxRuneLength(s string, maxLen int) error {
	return maxLength(utf8.RuneCountInString(s), maxLen)
}

// lengthBetween checks a non-empty string's byte length; "" passes so that required reports it.
func lengthBetween(s string, minLen, maxLen int) error {
	if s != "" && (len(s) < minLen || len(s) > maxLen) {
		return fmt.Errorf("the length must be between %d and %d", minLen, maxLen)
	}
	return nil
}

// matches accepts "" so that required reports a missing value.
func matches(s string, re *regexp.Regexp) error {
	if s != "" && !re.MatchString(s) {
		return errFormat
	}
	return nil
}

// atLeast and atMost compare in the accepting direction so NaN is rejected.
func atLeast[T int | float64](v, minValue T) error {
	if v >= minValue {
		return nil
	}
	return fmt.Errorf("must be no less than %v", minValue)
}

func atMost[T int | float64](v, maxValue T) error {
	if v <= maxValue {
		return nil
	}
	return fmt.Errorf("must be no greater than %v", maxValue)
}

func between[T int | float64](v, minValue, maxValue T) error {
	return firstErr(atLeast(v, minValue), atMost(v, maxValue))
}

// betweenIfSet accepts nil (field not sent).
func betweenIfSet(p *int, minValue, maxValue int) error {
	if p == nil {
		return nil
	}
	return between(*p, minValue, maxValue)
}

// validateIfSet runs a nested Validate; nil (field not sent) passes.
func validateIfSet[T interface{ Validate() error }](p *T) error {
	if p == nil {
		return nil
	}
	return (*p).Validate()
}

// validateEach checks every item and reports failures keyed by index.
func validateEach[T any](items []T, validate func(T) error) error {
	errs := fieldErrors{}
	for i, item := range items {
		errs.add(strconv.Itoa(i), validate(item))
	}
	return errs.err()
}
