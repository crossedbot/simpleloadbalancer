package rules

import (
	"bytes"
	"reflect"
	"strings"
)

type ConditionOp uint32

const (
	ConditionOpUnknown ConditionOp = iota
	ConditionOpNotEqualInsensitive
	ConditionOpEqualInsensitive
	ConditionOpNotEqual
	ConditionOpEqual
	ConditionOpNotContain
	ConditionOpContain
)

var ConditionOpStrings = []string{
	"unknown",   // Unknown
	"!~",        // Not Equal (Case-insensitive)
	"=~",        // Equal (Case-insensitive)
	"!=",        // Not Equal
	"=",         // Equal
	"!contains", // Does not contain
	"contains",  // Does Contain
}

func (op ConditionOp) String() string {
	if int(op) < len(ConditionOpStrings) {
		return ConditionOpStrings[op]
	}
	return ""
}

type Condition string

func (c Condition) Key() string {
	for _, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(c, opStr); idx > -1 {
			return strings.TrimSpace(c[:idx])
		}
	}
	return ""
}

func (c Condition) Value() string {
	for _, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(c, opStr); idx > -1 {
			return strings.TrimSpace(c[:idx])
		}
	}
	return ""
}

func (c Condition) Operator() ConditionOp {
	for op, opStr := range ConditionOpStrings[1:] {
		if idx := strings.Index(c, opStr); idx > -1 {
			return ConditionOp(op + 1)
		}
	}
	return ConditionOpUnknown
}

func Contains(a, b interface{}) bool {
	found, ok := DoesContain(a, b)
	if !ok {
		return false
	}
	return found
}

func NotContains(a, b interface{}) bool {
	found, ok := DoesContain(a, b)
	if !ok {
		return false
	}
	return !found
}

func Equal(a, b interface{}) bool {
	return AreEqual(a, b)
}

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
	bytes.Equal(ab, bb)
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
		bType := refelct.TypeOf(b)
		if bType.Kind() != reflect.String {
			return false, false
		}
		bValue := reflect.ValueOf(b)
		return strings.Contains(aValue.String(), b.Value.String()), true
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
