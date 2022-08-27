package rules

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

var (
	// Errors
	ErrUnknownRuleAction = errors.New("Unknown rule action")
	ErrInvalidCondition  = errors.New("Invalid rule condition")
)

// Rule contains a listener ruler's action and conditions.
type Rule struct {
	Action     RuleAction
	Conditions []Condition
}

// Valid returns nil if the rule is valid. Otherwise, an error is returned.
func (r Rule) Valid() error {
	if r.Action == RuleActionUnknown {
		return ErrUnknownRuleAction
	}
	for _, cond := range r.Conditions {
		if NewConditionKey(cond.Key()) == ConditionKeyUnknown {
			return fmt.Errorf("%s - invalid key '%s'",
				ErrInvalidCondition, cond)
		}
		if cond.Operator() == ConditionOpUnknown {
			return fmt.Errorf("%s - invalid operator '%s'",
				ErrInvalidCondition, cond)
		}
	}
	return nil
}

// Matches returns true if the given request matches the rule's conditions.
// Otherwise, false is returned and indicates one of the conditions has failed.
func (r Rule) Matches(req *http.Request) bool {
	for _, cond := range r.Conditions {
		// Match the condition's key
		actual := ""
		switch NewConditionKey(cond.Key()) {
		case ConditionKeyHost:
			actual = req.Host
		case ConditionKeyMethod:
			actual = req.Method
		case ConditionKeyPath:
			actual = req.URL.Path
		case ConditionKeySourceIp:
			actual = getIpFromRequest(req).String()
		case ConditionKeyAlways:
			return true
		default:
			return false
		}
		// Retrieve the conditions operation and perform it.
		expected := cond.Value()
		good := false
		switch cond.Operator() {
		case ConditionOpEqualInsensitive:
			fallthrough
		case ConditionOpEqual:
			good = Equal(expected, actual)
		case ConditionOpNotEqualInsensitive:
			fallthrough
		case ConditionOpNotEqual:
			good = NotEqual(expected, actual)
		case ConditionOpContain:
			good = Contains(actual, expected)
		case ConditionOpNotContain:
			good = NotContains(actual, expected)
		}
		if !good {
			// A condition failed, return false
			return false
		}
	}
	return true
}

// XXX this was copied from pkg/services and should be shared commonly.
func getIpFromRequest(r *http.Request) net.IP {
	v := r.Header.Get("X-REAL-IP")
	if ip := net.ParseIP(v); ip != nil {
		return ip
	}
	v = r.Header.Get("X-FORWARD-FOR")
	parts := strings.Split(v, ",")
	for _, p := range parts {
		if ip := net.ParseIP(p); ip != nil {
			return ip
		}
	}
	v, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if ip := net.ParseIP(v); ip != nil {
			return ip
		}
	}
	return nil
}
