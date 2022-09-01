package rules

import (
	"bytes"
	"reflect"
	"strings"
)

// ConditionOp represents a condition operator.
type ConditionOp uint32

const (
	// Condition operators
	ConditionOpUnknown ConditionOp = iota
	ConditionNoOp
	ConditionOpNotEqualInsensitive
	ConditionOpEqualInsensitive
	ConditionOpNotEqual
	ConditionOpEqual
	ConditionOpNotContain
	ConditionOpContain
)

// ConditionOpStrings is a list of string representations for condition
// operators.
var ConditionOpStrings = []string{
	"unknown",   // Unknown
	";",         // No Operation
	"!~",        // Not Equal (Case-insensitive)
	"=~",        // Equal (Case-insensitive)
	"!=",        // Not Equal
	"=",         // Equal
	"!contains", // Does not contain
	"contains",  // Does Contain
}

// String returns the string representation of the condition operator.
func (op ConditionOp) String() string {
	i := int(op)
	if i < len(ConditionOpStrings) {
		return ConditionOpStrings[i]
	}
	return ""
}

// Condition represents a rule's condition string.
type Condition string

// Key returns the key part of the condition statement.
func (c Condition) Key() string {
	for _, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(string(c), opStr); idx > -1 {
			return strings.TrimSpace(string(c[:idx]))
		}
	}
	return ""
}

// Value returns the value part of the condition statement.
func (c Condition) Value() string {
	for _, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(string(c), opStr); idx > -1 {
			s := string(c[idx:])
			s = strings.TrimPrefix(s, opStr)
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// Operator returns the condition operator of the condition statement.
func (c Condition) Operator() ConditionOp {
	for op, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(string(c), opStr); idx > -1 {
			return ConditionOp(op + 1)
		}
	}
	return ConditionOpUnknown
}

// Contains returns true if the given list 'a' contains element 'b'.
func Contains(a, b interface{}) bool {
	found, ok := DoesContain(a, b)
	if !ok {
		return false
	}
	return found
}

// NotContains returns true if the given list 'a' does not contain element 'b'.
func NotContains(a, b interface{}) bool {
	found, ok := DoesContain(a, b)
	if !ok {
		return true
	}
	return !found
}

// Equal returns true if the given value 'a' equals value 'b'.
func Equal(a, b interface{}) bool {
	return AreEqual(a, b)
}

// NotEqual returns true if the given value 'a' does not equal value 'b'.
func NotEqual(a, b interface{}) bool {
	return !AreEqual(a, b)
}

// AreEqual returns true if the two given objects are equal to one another. This
// routine performs a byte-by-byte comparison.
func AreEqual(a, b interface{}) bool {
	ab, ok := a.([]byte)
	if !ok {
		return reflect.DeepEqual(a, b)
	}
	bb, ok := b.([]byte)
	if !ok {
		return false
	}
	return bytes.Equal(ab, bb)
}

// DoesContain checks if the given list 'a' contains element 'b'. Returns
// whether the element was found and is OK.
func DoesContain(a, b interface{}) (bool, bool) {
	aValue := reflect.ValueOf(a)
	aType := reflect.TypeOf(a)
	if aType == nil {
		return false, false
	}
	aKind := aType.Kind()

	if aKind == reflect.String {
		bType := reflect.TypeOf(b)
		if bType.Kind() != reflect.String {
			return false, false
		}
		bValue := reflect.ValueOf(b)
		return strings.Contains(aValue.String(), bValue.String()), true
	}

	// XXX Probably not needed for this application
	if aKind == reflect.Map {
		keys := aValue.MapKeys()
		for _, key := range keys {
			if AreEqual(key.Interface(), b) {
				return true, true
			}
		}
		return false, true
	}

	if aKind == reflect.Slice || aKind == reflect.Array {
		length := aValue.Len()
		for i := 0; i < length; i++ {
			if AreEqual(aValue.Index(i).Interface(), b) {
				return true, true
			}
		}
	}

	return false, true
}
