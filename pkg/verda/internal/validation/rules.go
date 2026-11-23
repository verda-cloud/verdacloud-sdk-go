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

package validation

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"unicode/utf8"
)

// Every rule except Required, NilOrNotEmpty, Min, Max and By accepts an
// empty value, leaving it to Required to reject.

var (
	errBlank        = errors.New("cannot be blank")
	errInvalidValue = errors.New("must be a valid value")
	errFormat       = errors.New("must be in a valid format")
)

// RequiredRule rejects empty values.
type RequiredRule struct{ skipNil bool }

var (
	// Required rejects nil, zero values, empty strings/slices/maps and pointers to them.
	Required = RequiredRule{}
	// NilOrNotEmpty accepts nil but rejects a non-nil empty value.
	NilOrNotEmpty = RequiredRule{skipNil: true}
)

func (r RequiredRule) Validate(value any) error {
	value, isNil := indirect(value)
	if isNil && r.skipNil {
		return nil
	}
	if isEmpty(value) {
		return errBlank
	}
	return nil
}

// LengthRule checks the length of a string (bytes, or runes), slice, array or map.
type LengthRule struct {
	min, max int
	runes    bool
}

// Length checks min <= len <= max; 0 disables a bound.
func Length(minLen, maxLen int) LengthRule { return LengthRule{min: minLen, max: maxLen} }

// RuneLength is Length counting a string's runes instead of its bytes.
func RuneLength(minLen, maxLen int) LengthRule {
	return LengthRule{min: minLen, max: maxLen, runes: true}
}

func (r LengthRule) Validate(value any) error {
	value, _ = indirect(value)
	if isEmpty(value) {
		return nil
	}
	var n int
	if s, ok := value.(string); ok && r.runes {
		n = utf8.RuneCountInString(s)
	} else {
		v := reflect.ValueOf(value)
		switch v.Kind() {
		case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
			n = v.Len()
		default:
			return fmt.Errorf("cannot get the length of %v", v.Kind())
		}
	}
	if r.min > 0 && n < r.min || r.max > 0 && n > r.max || r.min == 0 && r.max == 0 {
		return r.err()
	}
	return nil
}

func (r LengthRule) err() error {
	switch {
	case r.min == 0 && r.max > 0:
		return fmt.Errorf("the length must be no more than %d", r.max)
	case r.min > 0 && r.max == 0:
		return fmt.Errorf("the length must be no less than %d", r.min)
	case r.min > 0 && r.min == r.max:
		return fmt.Errorf("the length must be exactly %d", r.min)
	case r.min > 0 && r.max > 0:
		return fmt.Errorf("the length must be between %d and %d", r.min, r.max)
	default:
		return errors.New("the value must be empty")
	}
}

// InRule accepts only the listed values.
type InRule struct{ elements []any }

// In accepts values deep-equal to one of elements.
func In(elements ...any) InRule { return InRule{elements: elements} }

func (r InRule) Validate(value any) error {
	value, _ = indirect(value)
	if isEmpty(value) {
		return nil
	}
	for _, e := range r.elements {
		if reflect.DeepEqual(e, value) {
			return nil
		}
	}
	return errInvalidValue
}

// ThresholdRule compares a number against a bound. Unlike the other rules it
// checks zero values; only nil is skipped.
type ThresholdRule struct {
	threshold any
	atLeast   bool
}

// Min requires value >= minValue. The value's kind must match the threshold's (int or float).
func Min(minValue any) ThresholdRule { return ThresholdRule{threshold: minValue, atLeast: true} }

// Max requires value <= maxValue. The value's kind must match the threshold's (int or float).
func Max(maxValue any) ThresholdRule { return ThresholdRule{threshold: maxValue} }

func (r ThresholdRule) Validate(value any) error {
	value, isNil := indirect(value)
	if isNil {
		return nil
	}
	t, v := reflect.ValueOf(r.threshold), reflect.ValueOf(value)
	var ok bool
	switch {
	case isInt(t.Kind()):
		if !isInt(v.Kind()) {
			return fmt.Errorf("cannot convert %v to int64", v.Kind())
		}
		ok = withinBound(v.Int(), t.Int(), r.atLeast)
	case isFloat(t.Kind()):
		if !isFloat(v.Kind()) {
			return fmt.Errorf("cannot convert %v to float64", v.Kind())
		}
		ok = withinBound(v.Float(), t.Float(), r.atLeast)
	default:
		return fmt.Errorf("type not supported: %v", t.Type())
	}
	if ok {
		return nil
	}
	if r.atLeast {
		return fmt.Errorf("must be no less than %v", r.threshold)
	}
	return fmt.Errorf("must be no greater than %v", r.threshold)
}

// withinBound compares in the accepting direction so that NaN fails.
func withinBound[T int64 | float64](value, bound T, atLeast bool) bool {
	if atLeast {
		return value >= bound
	}
	return value <= bound
}

func isInt(k reflect.Kind) bool { return k >= reflect.Int && k <= reflect.Int64 }

func isFloat(k reflect.Kind) bool { return k == reflect.Float32 || k == reflect.Float64 }

// MatchRule requires a string to match a regular expression.
type MatchRule struct {
	re  *regexp.Regexp
	err error
}

// Match requires a non-empty string to match re.
func Match(re *regexp.Regexp) MatchRule { return MatchRule{re: re, err: errFormat} }

// Error returns a copy of the rule that fails with message.
func (r MatchRule) Error(message string) MatchRule {
	r.err = errors.New(message)
	return r
}

func (r MatchRule) Validate(value any) error {
	value, isNil := indirect(value)
	if isNil {
		return nil
	}
	if s, ok := value.(string); ok && (s == "" || r.re.MatchString(s)) {
		return nil
	}
	return r.err
}

// indirect dereferences pointers and interfaces; isNil reports a nil value,
// pointer, interface, slice or map.
func indirect(value any) (_ any, isNil bool) {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Invalid:
		return nil, true
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil, true
		}
		return indirect(rv.Elem().Interface())
	case reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		if rv.IsNil() {
			return nil, true
		}
	}
	return value, false
}

// isEmpty reports nil, zero numbers, false, empty strings/slices/maps/arrays
// and zero structs. value must come from indirect, which strips pointers and
// turns every nil into an untyped nil.
func isEmpty(value any) bool {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Invalid:
		return true
	case reflect.Struct:
		return v.IsZero()
	}
	return false
}
