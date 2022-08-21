package rules

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConditionOpString(t *testing.T) {
	op := ConditionOpEqualInsensitive
	expected := ConditionOpStrings[int(op)]
	actual := op.String()
	require.Equal(t, expected, actual)

	op = ConditionOpNotEqual
	expected = ConditionOpStrings[int(op)]
	actual = op.String()
	require.Equal(t, expected, actual)

	op = ConditionOpNotContain
	expected = ConditionOpStrings[int(op)]
	actual = op.String()
	require.Equal(t, expected, actual)
}

func TestConditionKey(t *testing.T) {
	expected := "my_key"
	condition := Condition(fmt.Sprintf("%s=~some_value", expected))
	actual := condition.Key()
	require.Equal(t, expected, actual)

	condition = Condition(fmt.Sprintf("%s != some_value", expected))
	actual = condition.Key()
	require.Equal(t, expected, actual)
}

func TestConditionValue(t *testing.T) {
	expected := "some_value"
	condition := Condition(fmt.Sprintf("my_key=~%s", expected))
	actual := condition.Value()
	require.Equal(t, expected, actual)

	condition = Condition(fmt.Sprintf("my_key != %s", expected))
	actual = condition.Value()
	require.Equal(t, expected, actual)
}

func TestConditionOperator(t *testing.T) {
	expected := ConditionOpEqualInsensitive
	condition := Condition(fmt.Sprintf("my_key%ssome_value", expected))
	actual := condition.Operator()
	require.Equal(t, expected, actual)

	condition = Condition(fmt.Sprintf("my_key %s some_value", expected))
	actual = condition.Operator()
	require.Equal(t, expected, actual)
}

func TestAreEqual(t *testing.T) {
	i1 := 2
	i2 := 3
	require.False(t, AreEqual(i1, i2))
	i1 = i2
	require.True(t, AreEqual(i1, i2))

	f1 := 1.2
	f2 := 3.4
	require.False(t, AreEqual(f1, f2))
	f1 = f2
	require.True(t, AreEqual(f1, f2))

	c1 := 'a'
	c2 := 'b'
	require.False(t, AreEqual(c1, c2))
	c1 = c2
	require.True(t, AreEqual(c1, c2))

	s1 := "abc"
	s2 := "def"
	require.False(t, AreEqual(s1, s2))
	s1 = s2
	require.True(t, AreEqual(s1, s2))

	b1 := []byte("abc")
	b2 := []byte("def")
	require.False(t, AreEqual(b1, b2))
	b1 = b2
	require.True(t, AreEqual(b1, b2))
}

func TestDoesContainString(t *testing.T) {
	list := "hello world"
	found, ok := DoesContain(list, 123)
	require.False(t, ok)
	require.False(t, found)

	list = "hello world"
	elem := "today"
	found, ok = DoesContain(list, elem)
	require.True(t, ok)
	require.False(t, found)

	list = "hello world"
	elem = "world"
	found, ok = DoesContain(list, elem)
	require.True(t, ok)
	require.True(t, found)
}

func TestDoesContainMap(t *testing.T) {
	list := map[string]int{"hello": 123}
	found, ok := DoesContain(list, 123)
	require.True(t, ok)
	require.False(t, found)

	elem := "hello"
	found, ok = DoesContain(list, elem)
	require.True(t, ok)
	require.True(t, found)
}

func TestDoesContainSlice(t *testing.T) {
	list := []string{"hello", "world"}
	found, ok := DoesContain(list, 123)
	require.True(t, ok)
	require.False(t, found)

	elem := "today"
	found, ok = DoesContain(list, elem)
	require.True(t, ok)
	require.False(t, found)

	elem = "world"
	found, ok = DoesContain(list, elem)
	require.True(t, ok)
	require.True(t, found)
}

func TestContainsString(t *testing.T) {
	list := "hello world"
	elem := "today"
	require.False(t, Contains(list, elem))

	list = "hello world"
	elem = "world"
	require.True(t, Contains(list, elem))
}

func TestContainsMap(t *testing.T) {
	list := map[string]int{"hello": 123}
	require.False(t, Contains(list, 123))

	elem := "hello"
	require.True(t, Contains(list, elem))
}

func TestContainsSlice(t *testing.T) {
	list := []string{"hello", "world"}
	require.False(t, Contains(list, 123))

	elem := "today"
	require.False(t, Contains(list, elem))

	elem = "world"
	require.True(t, Contains(list, elem))
}

func TestNotContainsString(t *testing.T) {
	list := "hello world"
	elem := "today"
	require.True(t, NotContains(list, elem))

	list = "hello world"
	elem = "world"
	require.False(t, NotContains(list, elem))
}

func TestNotContainsMap(t *testing.T) {
	list := map[string]int{"hello": 123}
	require.True(t, NotContains(list, 123))

	elem := "hello"
	require.False(t, NotContains(list, elem))
}

func TestNotContainsSlice(t *testing.T) {
	list := []string{"hello", "world"}
	require.True(t, NotContains(list, 123))

	elem := "today"
	require.True(t, NotContains(list, elem))

	elem = "world"
	require.False(t, NotContains(list, elem))
}

func TestEqual(t *testing.T) {
	i1 := 2
	i2 := 3
	require.False(t, Equal(i1, i2))
	i1 = i2
	require.True(t, Equal(i1, i2))

	f1 := 1.2
	f2 := 3.4
	require.False(t, Equal(f1, f2))
	f1 = f2
	require.True(t, Equal(f1, f2))

	c1 := 'a'
	c2 := 'b'
	require.False(t, Equal(c1, c2))
	c1 = c2
	require.True(t, Equal(c1, c2))

	s1 := "abc"
	s2 := "def"
	require.False(t, Equal(s1, s2))
	s1 = s2
	require.True(t, Equal(s1, s2))

	b1 := []byte("abc")
	b2 := []byte("def")
	require.False(t, Equal(b1, b2))
	b1 = b2
	require.True(t, Equal(b1, b2))
}

func TestNotEqual(t *testing.T) {
	i1 := 2
	i2 := 3
	require.True(t, NotEqual(i1, i2))
	i1 = i2
	require.False(t, NotEqual(i1, i2))

	f1 := 1.2
	f2 := 3.4
	require.True(t, NotEqual(f1, f2))
	f1 = f2
	require.False(t, NotEqual(f1, f2))

	c1 := 'a'
	c2 := 'b'
	require.True(t, NotEqual(c1, c2))
	c1 = c2
	require.False(t, NotEqual(c1, c2))

	s1 := "abc"
	s2 := "def"
	require.True(t, NotEqual(s1, s2))
	s1 = s2
	require.False(t, NotEqual(s1, s2))

	b1 := []byte("abc")
	b2 := []byte("def")
	require.True(t, NotEqual(b1, b2))
	b1 = b2
	require.False(t, NotEqual(b1, b2))
}
