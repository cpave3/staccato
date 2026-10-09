package split

import (
	"strings"
	"testing"
)

func TestParsePlanRejectsUnknownField(t *testing.T) {
	_, err := ParsePlan([]byte(`{"slices": [{"name": "a", "path": ["x"]}]}`))
	if err == nil || !strings.Contains(err.Error(), `"path"`) {
		t.Fatalf("err = %v, want it to name the unknown field", err)
	}
}

func TestParsePlanErrors(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want []string
	}{
		{
			"comment",
			"{\n  \"slices\": [\n    // first the core\n    {\"name\": \"a\"}\n  ]\n}",
			[]string{"line 3", "comments are not allowed"},
		},
		{
			"trailing_comma",
			"{\n  \"slices\": [\n    {\"name\": \"a\"},\n  ]\n}",
			[]string{"line 4"},
		},
		{
			"second_document",
			`{"slices": []} {"slices": []}`,
			[]string{"after the plan"},
		},
		{
			"wrong_type",
			`{"slices": [{"name": "a", "paths": "core/"}]}`,
			[]string{"paths"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePlan([]byte(tc.plan))
			if err == nil {
				t.Fatal("ParsePlan accepted a malformed plan")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}

func TestPlanValidate(t *testing.T) {
	valid := `{"slices": [
		{"name": "a", "message": "first", "paths": ["x/"]},
		{"name": "b", "commits": ["abc123"]},
		{"name": "c", "message": "partial", "overrides": {"README.md": "r.md"}},
		{"name": "d", "message": "last", "rest": true}
	]}`
	plan, err := ParsePlan([]byte(valid))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}

	cases := []struct {
		name string
		plan string
		want string
	}{
		{
			"single_slice",
			`{"slices": [{"name": "a", "message": "m", "rest": true}]}`,
			"at least two slices",
		},
		{
			"missing_name",
			`{"slices": [{"message": "m", "paths": ["x"]}, {"name": "b", "message": "m", "rest": true}]}`,
			"slice 1 has no name",
		},
		{
			"duplicate_name",
			`{"slices": [{"name": "a", "message": "m", "paths": ["x"]}, {"name": "a", "message": "m", "rest": true}]}`,
			`"a" is used by more than one slice`,
		},
		{
			"no_content",
			`{"slices": [{"name": "a", "message": "m"}, {"name": "b", "message": "m", "rest": true}]}`,
			`slice "a" has no content`,
		},
		{
			"commits_and_paths",
			`{"slices": [{"name": "a", "commits": ["abc"], "paths": ["x"]}, {"name": "b", "message": "m", "rest": true}]}`,
			`slice "a" mixes`,
		},
		{
			"rest_and_paths",
			`{"slices": [{"name": "a", "message": "m", "paths": ["x"]}, {"name": "b", "message": "m", "paths": ["y"], "rest": true}]}`,
			`slice "b" mixes`,
		},
		{
			"paths_without_message",
			`{"slices": [{"name": "a", "paths": ["x"]}, {"name": "b", "message": "m", "rest": true}]}`,
			`slice "a" needs a message`,
		},
		{
			"rest_without_message",
			`{"slices": [{"name": "a", "message": "m", "paths": ["x"]}, {"name": "b", "rest": true}]}`,
			`slice "b" needs a message`,
		},
		{
			"commits_with_message",
			`{"slices": [{"name": "a", "message": "m", "commits": ["abc"]}, {"name": "b", "message": "m", "rest": true}]}`,
			`slice "a" has a message`,
		},
		{
			"rest_not_last",
			`{"slices": [{"name": "a", "message": "m", "rest": true}, {"name": "b", "message": "m", "paths": ["x"]}]}`,
			`only the last slice can use "rest"`,
		},
		{
			"empty_path",
			`{"slices": [{"name": "a", "message": "m", "paths": [""]}, {"name": "b", "message": "m", "rest": true}]}`,
			`slice "a" has an empty path`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := ParsePlan([]byte(tc.plan))
			if err != nil {
				t.Fatalf("ParsePlan: %v", err)
			}
			err = plan.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
