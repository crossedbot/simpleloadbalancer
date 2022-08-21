package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRuleAction(t *testing.T) {
	expected := RuleActionForward
	s := RuleActionStrings[int(expected)]
	actual := NewRuleAction(s)
	require.Equal(t, expected, actual)
}

func TestRuleActionString(t *testing.T) {
	action := RuleActionForward
	expected := RuleActionStrings[int(action)]
	actual := action.String()
	require.Equal(t, expected, actual)
}
