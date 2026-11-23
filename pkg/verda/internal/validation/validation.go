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

// Package validation is the subset of the github.com/go-ozzo/ozzo-validation/v4
// API that the SDK uses, with the same semantics and error messages, so the
// SDK carries no third-party dependency. Add a rule here only when a Validate
// method needs it.
//
// One difference from ozzo: a nil pointer element in a slice, array or map of
// Validatable is skipped, where ozzo calls Validate on it and a value-receiver
// Validate panics.
//
// Not ported, so a Validate method that needs one must add it here first:
// Else, Skip, Exclusive on Min/Max, uint and time.Time thresholds for Min/Max,
// Error (a custom message) on any rule but Match, []byte for Match,
// driver.Valuer unwrapping, embedded-struct field lookup, error codes, and a
// configurable ErrorTag (field names always come from the json tag).
package validation

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Rule checks one value.
type Rule interface {
	Validate(value any) error
}

// Validatable is a value that validates itself; Validate calls it after a
// field's rules pass.
type Validatable interface {
	Validate() error
}

var validatableType = reflect.TypeFor[Validatable]()

// Errors maps a field name (or slice index, or map key) to its error.
type Errors map[string]error

// Error lists entries sorted by key and wraps nested Errors in parentheses:
// "a: cannot be blank; b: (c: must be a valid value.)."
func (es Errors) Error() string {
	if len(es) == 0 {
		return ""
	}
	var s strings.Builder
	for i, key := range slices.Sorted(maps.Keys(es)) {
		if i > 0 {
			s.WriteString("; ")
		}
		s.WriteString(key + ": ")
		if nested, ok := es[key].(Errors); ok {
			s.WriteString("(" + nested.Error() + ")")
		} else {
			s.WriteString(es[key].Error())
		}
	}
	s.WriteString(".")
	return s.String()
}

// MarshalJSON emits each entry's message, and nested Errors as objects, so
// json.Marshal of a validation error carries the text instead of "{}".
func (es Errors) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(es))
	for key, err := range es {
		if m, ok := err.(json.Marshaler); ok {
			out[key] = m
		} else {
			out[key] = err.Error()
		}
	}
	return json.Marshal(out)
}

// orNil returns es as an error, or an untyped nil when it holds no failure.
func (es Errors) orNil() error {
	if len(es) == 0 {
		return nil
	}
	return es
}

// FieldRules binds rules to one struct field, given by pointer.
type FieldRules struct {
	fieldPtr any
	rules    []Rule
}

// Field binds rules to the struct field that fieldPtr points at.
func Field(fieldPtr any, rules ...Rule) *FieldRules {
	return &FieldRules{fieldPtr: fieldPtr, rules: rules}
}

// ValidateStruct validates the given fields of the struct structPtr points at.
// Failures are keyed by the field's json tag name, or its Go name when untagged.
// A field that is not in the struct is a programming error and is returned as is.
func ValidateStruct(structPtr any, fields ...*FieldRules) error {
	sv := reflect.ValueOf(structPtr)
	if sv.Kind() != reflect.Pointer || !sv.IsNil() && sv.Elem().Kind() != reflect.Struct {
		return errors.New("only a pointer to a struct can be validated")
	}
	if sv.IsNil() {
		return nil
	}
	sv = sv.Elem()

	errs := Errors{}
	for i, fr := range fields {
		fv := reflect.ValueOf(fr.fieldPtr)
		if fv.Kind() != reflect.Pointer {
			return fmt.Errorf("field #%d must be specified as a pointer", i)
		}
		name, ok := fieldName(sv, fv)
		if !ok {
			return fmt.Errorf("field #%d cannot be found in the struct", i)
		}
		if err := Validate(fv.Elem().Interface(), fr.rules...); err != nil {
			errs[name] = err
		}
	}
	return errs.orNil()
}

// fieldName finds the top-level field of sv at the address fieldPtr holds.
// Embedded structs are not searched: no SDK request type embeds one.
func fieldName(sv, fieldPtr reflect.Value) (string, bool) {
	for i := range sv.NumField() {
		sf := sv.Type().Field(i)
		if sv.Field(i).UnsafeAddr() != fieldPtr.Pointer() || sf.Type != fieldPtr.Elem().Type() {
			continue
		}
		if tag, _, _ := strings.Cut(sf.Tag.Get("json"), ","); tag != "" && tag != "-" {
			return tag, true
		}
		return sf.Name, true
	}
	return "", false
}

// Validate applies rules in order and stops at the first failure. When they
// all pass, it validates value itself if it is a Validatable, or each element
// if it is a slice, array or map of Validatable; nil pointers are skipped.
func Validate(value any, rules ...Rule) error {
	for _, rule := range rules {
		if err := rule.Validate(value); err != nil {
			return err
		}
	}

	rv := reflect.ValueOf(value)
	if isNilReference(rv) {
		return nil
	}
	if v, ok := value.(Validatable); ok {
		return v.Validate()
	}

	switch rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		if rv.Type().Elem().Implements(validatableType) {
			return validateElements(rv, nil)
		}
	case reflect.Pointer, reflect.Interface:
		return Validate(rv.Elem().Interface())
	}
	return nil
}

// validateElements runs Validate with rules on every element of a map, slice
// or array, keying each failure by map key or index. A nil pointer or
// interface element reaches the rules as an untyped nil.
func validateElements(rv reflect.Value, rules []Rule) error {
	errs := Errors{}
	check := func(key any, elem reflect.Value) {
		var value any
		if !isNilReference(elem) {
			value = elem.Interface()
		}
		if err := Validate(value, rules...); err != nil {
			errs[fmt.Sprint(key)] = err
		}
	}
	switch rv.Kind() {
	case reflect.Map:
		for iter := rv.MapRange(); iter.Next(); {
			check(iter.Key().Interface(), iter.Value())
		}
	case reflect.Slice, reflect.Array:
		for i := range rv.Len() {
			check(i, rv.Index(i))
		}
	default:
		return errors.New("must be an iterable (map, slice or array)")
	}
	return errs.orNil()
}

func isNilReference(v reflect.Value) bool {
	return (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil()
}

// RuleFunc is a custom check for By.
type RuleFunc func(value any) error

type funcRule struct{ f RuleFunc }

func (r funcRule) Validate(value any) error { return r.f(value) }

// By turns a function into a rule. The function also sees empty values.
func By(f RuleFunc) Rule { return funcRule{f: f} }

// WhenRule applies its rules only when its condition holds.
type WhenRule struct {
	condition bool
	rules     []Rule
}

// When applies rules only if condition is true.
func When(condition bool, rules ...Rule) WhenRule {
	return WhenRule{condition: condition, rules: rules}
}

func (r WhenRule) Validate(value any) error {
	if !r.condition {
		return nil
	}
	return Validate(value, r.rules...)
}

// EachRule applies its rules to every element of a slice, array or map.
type EachRule struct{ rules []Rule }

// Each applies rules to every element; failures are keyed by index or map key.
func Each(rules ...Rule) EachRule { return EachRule{rules: rules} }

func (r EachRule) Validate(value any) error {
	return validateElements(reflect.ValueOf(value), r.rules)
}
