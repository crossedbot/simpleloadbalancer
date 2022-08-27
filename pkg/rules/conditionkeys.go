package rules

import (
	"strings"
)

// ConditionKey is a numerical representation of a key for a listener rule's
// condition.
type ConditionKey uint32

const (
	// List of condition keys
	ConditionKeyUnknown ConditionKey = iota
	ConditionKeyHost
	ConditionKeyMethod
	ConditionKeyPath
	ConditionKeySourceIp
	ConditionKeyAlways
)

// ConditionKeyStrings is a list of string representations for condition keys.
var ConditionKeyStrings = []string{
	"unknown",
	"host-header",
	"http-request-method",
	"path-pattern",
	"source-ip",
	"always",
}

// NewConditionKey returns the ConditionKey for a given string. If the string
// does not match a known conditon key, ConditionKeyUnknown is returned.
func NewConditionKey(v string) ConditionKey {
	for idx, s := range ConditionKeyStrings {
		if strings.EqualFold(s, v) {
			return ConditionKey(idx)
		}
	}
	return ConditionKeyUnknown
}

// String returns the string representation for the given condition key.
func (k ConditionKey) String() string {
	i := int(k)
	if i > len(ConditionKeyStrings) {
		i = int(ConditionKeyUnknown)
	}
	return ConditionKeyStrings[i]
}
