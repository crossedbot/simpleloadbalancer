package rules

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConditionKey(t *testing.T) {
	expected := ConditionKeyMethod
	s := ConditionKeyStrings[int(expected)]
	actual := NewConditionKey(s)
	require.Equal(t, expected, actual)
}

func TestConditionKeyString(t *testing.T) {
	ck := ConditionKeyMethod
	expected := ConditionKeyStrings[int(ck)]
	actual := ck.String()
	require.Equal(t, expected, actual)
}
