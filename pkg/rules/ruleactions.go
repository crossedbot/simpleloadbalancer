package rules

import (
	"strings"
)

// RuleAction is a numerical representation of a listener rule's action.
type RuleAction uint32

const (
	// List of rule actions
	RuleActionUnknown RuleAction = iota
	RuleActionForward
	RuleActionRedirect
)

// RuleActionStrings is a list of the string representations of the rule
// actions.
var RuleActionStrings = []string{
	"unknown",
	"forward",
	"redirect",
}

// NewRuleAction returns the RuleAction for a given string. If the string does
// not match a known action, RuleActionUnknown is returned.
func NewRuleAction(v string) RuleAction {
	for idx, s := range RuleActionStrings {
		if strings.EqualFold(s, v) {
			return RuleAction(idx)
		}
	}
	return RuleActionUnknown
}

// String returns the string representation for the given action.
func (a RuleAction) String() string {
	i := int(a)
	if i > len(RuleActionStrings) {
		i = int(RuleActionUnknown)
	}
	return RuleActionStrings[i]
}
