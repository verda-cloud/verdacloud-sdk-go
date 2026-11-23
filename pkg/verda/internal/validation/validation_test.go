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
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"testing"
)

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type item struct {
	Name string `json:"name"`
}

func (i item) Validate() error {
	return ValidateStruct(&i, Field(&i.Name, Required))
}

func TestRules(t *testing.T) {
	str := func(s string) *string { return &s }
	intPtr := func(v int) *int { return &v }
	tests := []struct {
		name  string
		value any
		rule  Rule
		want  string
	}{
		{"required string", "x", Required, ""},
		{"required empty string", "", Required, "cannot be blank"},
		{"required zero int", 0, Required, "cannot be blank"},
		{"required nil slice", []string(nil), Required, "cannot be blank"},
		{"required empty slice", []string{}, Required, "cannot be blank"},
		{"required nil pointer", (*string)(nil), Required, "cannot be blank"},
		{"required pointer to empty", str(""), Required, "cannot be blank"},
		{"required zero struct", item{}, Required, "cannot be blank"},
		{"required non-zero struct", item{Name: "a"}, Required, ""},
		{"nil or not empty nil", (*string)(nil), NilOrNotEmpty, ""},
		{"nil or not empty set", str("x"), NilOrNotEmpty, ""},
		{"nil or not empty pointer to empty", str(""), NilOrNotEmpty, "cannot be blank"},
		{"length skips empty", "", Length(1, 2), ""},
		{"length too long", "abc", Length(0, 2), "the length must be no more than 2"},
		{"length too short", "a", Length(2, 0), "the length must be no less than 2"},
		{"length out of range", "abcd", Length(1, 3), "the length must be between 1 and 3"},
		{"length exact", "abc", Length(2, 2), "the length must be exactly 2"},
		{"length of slice", []int{1, 2, 3}, Length(0, 2), "the length must be no more than 2"},
		{"length counts bytes", "ää", Length(0, 3), "the length must be no more than 3"},
		{"rune length counts runes", "ää", RuneLength(0, 3), ""},
		{"rune length too long", "äää", RuneLength(0, 2), "the length must be no more than 2"},
		{"length of int", 5, Length(0, 2), "cannot get the length of int"},
		{"in allowed", "a", In("a", "b"), ""},
		{"in skips empty", "", In("a"), ""},
		{"in rejected", "c", In("a", "b"), "must be a valid value"},
		{"min ok", 1, Min(1), ""},
		{"min checks zero", 0, Min(1), "must be no less than 1"},
		{"min skips nil", (*int)(nil), Min(1), ""},
		{"min through pointer", intPtr(-1), Min(0), "must be no less than 0"},
		{"max rejected", 11, Max(10), "must be no greater than 10"},
		{"float min", 0.5, Min(1.0), "must be no less than 1"},
		{"float max", 1000.5, Max(1000.0), "must be no greater than 1000"},
		{"float min NaN", math.NaN(), Min(1.0), "must be no less than 1"},
		{"int threshold float value", 1.5, Min(1), "cannot convert float64 to int64"},
		{"match ok", "ab", Match(regexp.MustCompile(`^a`)), ""},
		{"match skips empty", "", Match(regexp.MustCompile(`^a`)), ""},
		{"match rejected", "ba", Match(regexp.MustCompile(`^a`)), "must be in a valid format"},
		{"match custom error", "ba", Match(regexp.MustCompile(`^a`)).Error("must start with a"), "must start with a"},
		{"match custom error ok", "ab", Match(regexp.MustCompile(`^a`)).Error("must start with a"), ""},
		{"match custom error skips empty", "", Match(regexp.MustCompile(`^a`)).Error("must start with a"), ""},
		{"by", "x", By(func(any) error { return errors.New("custom") }), "custom"},
		{"when true", 11, When(true, Max(10)), "must be no greater than 10"},
		{"when false", 11, When(false, Max(10)), ""},
		{"each ok", []string{"a"}, Each(Required), ""},
		{"each by index", []string{"a", ""}, Each(Required), "1: cannot be blank."},
		{"each map key", map[string]string{"k": ""}, Each(Required), "k: cannot be blank."},
		{"each not iterable", 1, Each(Required), "must be an iterable (map, slice or array)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errText(tc.rule.Validate(tc.value)); got != tc.want {
				t.Errorf("Validate(%#v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestMatchRule_ErrorLeavesTheOriginalRuleUnchanged(t *testing.T) {
	base := Match(regexp.MustCompile(`^a`))
	_ = base.Error("custom")
	if got := errText(base.Validate("ba")); got != "must be in a valid format" {
		t.Errorf("Validate() = %q, want the default message", got)
	}
}

func TestValidateStruct_FieldNames(t *testing.T) {
	s := struct {
		Tagged    string `json:"tagged_name,omitempty"`
		Untagged  string
		Dash      string `json:"-"`
		OmitEmpty string `json:",omitempty"`
	}{}
	err := ValidateStruct(&s,
		Field(&s.Tagged, Required),
		Field(&s.Untagged, Required),
		Field(&s.Dash, Required),
		Field(&s.OmitEmpty, Required),
	)
	want := "Dash: cannot be blank; OmitEmpty: cannot be blank; Untagged: cannot be blank; tagged_name: cannot be blank."
	if got := errText(err); got != want {
		t.Errorf("ValidateStruct() = %q, want %q", got, want)
	}
	var errs Errors
	if !errors.As(err, &errs) || len(errs) != 4 {
		t.Errorf("ValidateStruct() = %#v, want Errors with 4 entries", err)
	}
}

func TestValidateStruct_StopsAtFirstFailurePerField(t *testing.T) {
	s := struct {
		Name string `json:"name"`
	}{}
	called := false
	err := ValidateStruct(&s, Field(&s.Name, Required, By(func(any) error { called = true; return nil })))
	if got := errText(err); got != "name: cannot be blank." {
		t.Errorf("ValidateStruct() = %q", got)
	}
	if called {
		t.Error("rule after the first failure ran")
	}
}

func TestValidateStruct_Valid(t *testing.T) {
	s := struct {
		Name string `json:"name"`
	}{Name: "a"}
	if err := ValidateStruct(&s, Field(&s.Name, Required)); err != nil {
		t.Errorf("ValidateStruct() = %v, want nil", err)
	}
}

func TestValidateStruct_NestedAutoValidation(t *testing.T) {
	s := struct {
		Value   item            `json:"value"`
		Ptr     *item           `json:"ptr"`
		NilPtr  *item           `json:"nil_ptr"`
		Slice   []item          `json:"slice"`
		Map     map[string]item `json:"map"`
		Skipped item            `json:"skipped"`
	}{
		Ptr:   &item{},
		Slice: []item{{Name: "ok"}, {}},
		Map:   map[string]item{"k": {}},
	}
	err := ValidateStruct(&s,
		Field(&s.Value),
		Field(&s.Ptr),
		Field(&s.NilPtr),
		Field(&s.Slice),
		Field(&s.Map),
		Field(&s.Skipped, By(func(any) error { return errors.New("rule failed") })), // hides the nested error
	)
	want := "map: (k: (name: cannot be blank.).); ptr: (name: cannot be blank.); " +
		"skipped: rule failed; slice: (1: (name: cannot be blank.).); value: (name: cannot be blank.)."
	if got := errText(err); got != want {
		t.Errorf("ValidateStruct() =\n%q\nwant\n%q", got, want)
	}
}

func TestValidateStruct_ProgrammingErrors(t *testing.T) {
	s := struct{ A string }{}
	other := ""
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"not a pointer", ValidateStruct(s), "only a pointer to a struct can be validated"},
		{"nil pointer", ValidateStruct((*struct{ A string })(nil)), ""},
		{"field not a pointer", ValidateStruct(&s, Field(s.A)), "field #0 must be specified as a pointer"},
		{"field not in struct", ValidateStruct(&s, Field(&other)), "field #0 cannot be found in the struct"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := errText(tc.err); got != tc.want {
				t.Errorf("ValidateStruct() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrors_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		errs Errors
		want string
	}{
		{"empty", Errors{}, `{}`},
		{"messages", Errors{"b": errors.New("x"), "a": errors.New("y")}, `{"a":"y","b":"x"}`},
		{"nested", Errors{"a": Errors{"0": errors.New("x")}}, `{"a":{"0":"x"}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.errs)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("json.Marshal() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestErrors_MarshalJSON_ThroughErrorInterface(t *testing.T) {
	s := struct {
		Name  string `json:"name"`
		Items []item `json:"items"`
	}{Items: []item{{}}}
	err := ValidateStruct(&s, Field(&s.Name, Required), Field(&s.Items))
	got, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("json.Marshal() error = %v", marshalErr)
	}
	want := `{"items":{"0":{"name":"cannot be blank"}},"name":"cannot be blank"}`
	if string(got) != want {
		t.Errorf("json.Marshal() = %s, want %s", got, want)
	}
}

func TestErrors_Error(t *testing.T) {
	tests := []struct {
		name string
		errs Errors
		want string
	}{
		{"empty", Errors{}, ""},
		{"sorted", Errors{"b": errors.New("x"), "a": errors.New("y")}, "a: y; b: x."},
		{"nested", Errors{"a": Errors{"0": errors.New("x")}}, "a: (0: x.)."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.errs.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
