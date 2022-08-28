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
	Conditions [][]Condition
}

// Valid returns nil if the rule is valid. Otherwise, an error is returned.
func (r Rule) Valid() error {
	if r.Action == RuleActionUnknown {
		return ErrUnknownRuleAction
	}
	for i, cond := range r.Conditions {
		for _, sub := range cond {
			if NewConditionKey(sub.Key()) == ConditionKeyUnknown {
				return fmt.Errorf(
					"%s - invalid key '%s' (%d)",
					ErrInvalidCondition, sub, i,
				)
			}
			if sub.Operator() == ConditionOpUnknown {
				return fmt.Errorf(
					"%s - invalid operator '%s' (%d)",
					ErrInvalidCondition, sub, i,
				)
			}
		}
	}
	return nil
}

// Matches returns true if the given request matches the rule's conditions.
// Otherwise, false is returned and indicates one of the conditions has failed.
func (r Rule) Matches(req *http.Request) bool {
	for _, cond := range r.Conditions {
		good := false
		for _, sub := range cond {
			if NewConditionKey(sub.Key()) == ConditionKeyAlways {
				// Override all conditions
				return true
			}
			if good = matchRequest(sub, req); good {
				break
			}
		}
		if !good {
			// All sub-conditions failed, return false
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

// match returns true if the actual string matches the expected string depending
// on the operation.
func match(expected, actual string, op ConditionOp) bool {
	switch op {
	case ConditionOpEqualInsensitive:
		fallthrough
	case ConditionOpEqual:
		return Equal(expected, actual)
	case ConditionOpNotEqualInsensitive:
		fallthrough
	case ConditionOpNotEqual:
		return NotEqual(expected, actual)
	case ConditionOpContain:
		return Contains(actual, expected)
	case ConditionOpNotContain:
		return NotContains(actual, expected)
	}
	return false
}

// matchCIDR returns true if the IP address string is contained or not contained
// in the given network range string depending on the operation.
func matchCIDR(netStr, ipStr string, op ConditionOp) bool {
	_, n, err := net.ParseCIDR(netStr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	contains := fmt.Sprintf("%t", NetworkContains(*n, ip))
	return match("true", contains, op)
}

// matchRequest returns true if the given request matches the given condition.
func matchRequest(cond Condition, req *http.Request) bool {
	actual := ""
	expected := cond.Value()
	op := cond.Operator()
	switch NewConditionKey(cond.Key()) {
	case ConditionKeyHost:
		actual = req.Host
		return match(expected, actual, op)
	case ConditionKeyMethod:
		actual = req.Method
		return match(expected, actual, op)
	case ConditionKeyPath:
		actual = req.URL.Path
		return match(expected, actual, op)
	case ConditionKeySourceIp:
		actual = getIpFromRequest(req).String()
		if IsCIDR(expected) {
			return matchCIDR(expected, actual, op)
		} else {
			return match(expected, actual, op)
		}
	case ConditionKeyAlways:
		return true
	}
	return false
}
