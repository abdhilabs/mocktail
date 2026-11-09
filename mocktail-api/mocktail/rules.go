package mocktail

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// RequestRule is evaluated in array order. The first matching rule supplies the response.
// Match values are exact strings unless present is set, which tests existence/absence.
type RequestRule struct {
	Query      map[string]ValueMatcher `json:"query,omitempty"`
	Headers    map[string]ValueMatcher `json:"headers,omitempty"`
	Method     string                  `json:"method,omitempty"`
	Path       string                  `json:"path,omitempty"`
	StatusCode int                     `json:"statusCode"`
	Response   json.RawMessage         `json:"response"`
	HeadersOut map[string]string       `json:"responseHeaders,omitempty"`
}

type ValueMatcher struct {
	Exact   *string `json:"exact,omitempty"`
	Present *bool   `json:"present,omitempty"`
}

// UnmarshalJSON accepts the documented matcher object and the concise legacy form
// {"query":{"mode":"preview"}}, treating the latter as an exact match.
func (m *ValueMatcher) UnmarshalJSON(data []byte) error {
	var exact string
	if err := json.Unmarshal(data, &exact); err == nil {
		m.Exact = &exact
		m.Present = nil
		return nil
	}
	type matcher ValueMatcher
	var parsed matcher
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*m = ValueMatcher(parsed)
	return nil
}

func ValidateRules(raw []byte) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var rules []RequestRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return fmt.Errorf("rules must be an array: %w", err)
	}
	for i, r := range rules {
		if r.Method != "" && !isAllowedMethod(strings.ToUpper(r.Method)) {
			return fmt.Errorf("rule %d: unsupported method %q", i+1, r.Method)
		}
		if r.StatusCode < 100 || r.StatusCode > 599 {
			return fmt.Errorf("rule %d: statusCode must be between 100 and 599", i+1)
		}
		if len(r.Response) == 0 || string(r.Response) == "null" || !json.Valid(r.Response) {
			return fmt.Errorf("rule %d: response must be valid JSON", i+1)
		}
		if err := validateMatchers(r.Query, "query", i); err != nil {
			return err
		}
		if err := validateMatchers(r.Headers, "headers", i); err != nil {
			return err
		}
	}
	return nil
}

func validateMatchers(m map[string]ValueMatcher, kind string, i int) error {
	for name, v := range m {
		if name == "" {
			return fmt.Errorf("rule %d: %s matcher name cannot be empty", i+1, kind)
		}
		if (v.Exact == nil) == (v.Present == nil) {
			return fmt.Errorf("rule %d: %s %q needs exactly one of exact or present", i+1, kind, name)
		}
	}
	return nil
}

func matchRule(c *fiber.Ctx, r RequestRule, path string) bool {
	if r.Method != "" && !strings.EqualFold(r.Method, c.Method()) {
		return false
	}
	if r.Path != "" && r.Path != path {
		return false
	}
	for name, matcher := range r.Query {
		if !matchValue(c.Queries()[name], c.Context().QueryArgs().Has(name), matcher) {
			return false
		}
	}
	for name, matcher := range r.Headers {
		value := c.Get(name)
		if !matchValue(value, value != "", matcher) {
			return false
		}
	}
	return true
}

func matchValue(value string, present bool, m ValueMatcher) bool {
	if m.Present != nil {
		return present == *m.Present
	}
	return present && value == *m.Exact
}

func isAllowedMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}
