package rules

import ()

type RuleAction uint32

const (
	RuleActionForward RuleAction = iota + 1
	RuleActionRedirect
)

type Rule struct {
	Action     string
	Conditions []Condition
}
