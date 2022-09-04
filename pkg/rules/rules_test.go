package rules

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		A        string
		B        string
		Op       ConditionOp
		Expected bool
	}{
		{"ABC", "ABC", ConditionOpEqual, true},
		{"ABC", "DEF", ConditionOpEqual, false},
		{"ABC", "DEF", ConditionOpNotEqual, true},
		{"ABC", "ABC", ConditionOpNotEqual, false},
		{"ello", "HelloWorld", ConditionOpContain, true},
		{"ABC", "HelloWorld", ConditionOpContain, false},
		{"ABC", "HelloWorld", ConditionOpNotContain, true},
		{"ello", "HelloWorld", ConditionOpNotContain, false},
	}
	for _, test := range tests {
		actual := match(test.A, test.B, test.Op)
		require.Equal(t, test.Expected, actual)
	}
}

func TestMatchCIDR(t *testing.T) {
	tests := []struct {
		A        string
		B        string
		Op       ConditionOp
		Expected bool
	}{
		{"192.168.0.0/24", "192.168.0.10", ConditionOpEqual, true},
		{"192.168.0.0/24", "127.0.0.1", ConditionOpEqual, false},
		{"192.168.0.0/24", "127.0.0.1", ConditionOpNotEqual, true},
		{"192.168.0.0/24", "192.168.0.10", ConditionOpNotEqual, false},
	}
	for _, test := range tests {
		actual := matchCIDR(test.A, test.B, test.Op)
		require.Equal(t, test.Expected, actual)
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		A        string
		B        string
		Op       ConditionOp
		Expected bool
	}{
		{"*", "/hello/world", ConditionOpEqual, true},
		{"/goodbye/world", "/hello/world", ConditionOpNotEqual, true},
		{"/hello", "/hello/world", ConditionOpContain, true},
		{"/hello/world", "/goodbye", ConditionOpNotContain, true},
		{"/hello", "/HELLO", ConditionOpEqualInsensitive, true},
		{"/good", "/bad", ConditionOpNotEqualInsensitive, true},
		{"/users/*", "/users/login", ConditionOpEqual, true},
		{"/user*/log??", "/users/login", ConditionOpEqual, true},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected,
			matchPath(test.A, test.B, test.Op))
	}
}

func TestMatchRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)

	cond := Condition("host-header = example.com")
	req.Header.Set("Host", "example.com")
	require.True(t, matchRequest(cond, req))
	req.Header.Set("Host", "notexample.com")
	require.False(t, matchRequest(cond, req))
	cond = Condition("host-header != example.com")
	require.True(t, matchRequest(cond, req))
	req.Header.Set("Host", "example.com")
	require.False(t, matchRequest(cond, req))

	cond = Condition("http-request-method = GET")
	req.Method = http.MethodGet
	require.True(t, matchRequest(cond, req))
	req.Method = http.MethodPost
	require.False(t, matchRequest(cond, req))
	cond = Condition("http-request-method != GET")
	require.True(t, matchRequest(cond, req))
	req.Method = http.MethodGet
	require.False(t, matchRequest(cond, req))

	cond = Condition("path-pattern = /users/login")
	req.URL.Path = "/users/login"
	require.True(t, matchRequest(cond, req))
	req.URL.Path = "/hello/world"
	require.False(t, matchRequest(cond, req))
	cond = Condition("path-pattern != /users/login")
	require.True(t, matchRequest(cond, req))
	req.URL.Path = "/users/login"
	require.False(t, matchRequest(cond, req))
	cond = Condition("path-pattern contains /users")
	require.True(t, matchRequest(cond, req))
	req.URL.Path = "/hello/world"
	require.False(t, matchRequest(cond, req))
	cond = Condition("path-pattern !contains /users")
	require.True(t, matchRequest(cond, req))
	req.URL.Path = "/users/login"
	require.False(t, matchRequest(cond, req))

	cond = Condition("source-ip = 127.0.0.0/24")
	req.RemoteAddr = net.JoinHostPort("127.0.0.10", "8080")
	require.True(t, matchRequest(cond, req))
	req.RemoteAddr = net.JoinHostPort("192.168.0.10", "8080")
	require.False(t, matchRequest(cond, req))
	cond = Condition("source-ip != 127.0.0.0/24")
	require.True(t, matchRequest(cond, req))
	req.RemoteAddr = net.JoinHostPort("127.0.0.10", "8080")
	require.False(t, matchRequest(cond, req))

	cond = Condition("always;")
	require.True(t, matchRequest(cond, req))
}

func TestMatchStrings(t *testing.T) {
	tests := []struct {
		Patt     string
		Str      string
		Expected bool
	}{
		{"*", "helloworld", true},
		{"hell*orld", "helloworld", true},
		{"h*world", "hello", false},
		{"*hello", "helloworld", false},
		{"he?lo*d", "helloworld", true},
		{"*elo*", "helloworld", false},
		{"*abc***/*d*e*f*/**gh**ij*k*", "aabc/def/ghijk", true},
		{"abc***/*d*e*f*/**gh**ij*k*", "aabc/def/ghijk", false},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected,
			matchStrings(test.Patt, test.Str))
	}
}

func TestRmRepeatRune(t *testing.T) {
	tests := []struct {
		Str      string
		Run      rune
		Expected string
	}{
		{"*****", '*', "*"},
		{"** ** **", '*', "* * *"},
		{"a***b", '*', "a*b"},
		{"***aaa", '*', "*aaa"},
		{"abc", '*', "abc"},
		{"abc***", '*', "abc*"},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected,
			rmRepeatRune(test.Str, test.Run))
	}
}

func TestRuleValid(t *testing.T) {
	rule := Rule{
		Action: RuleActionForward,
		Conditions: [][]Condition{
			{Condition("source-ip=127.0.0.1")},
			{Condition("path-pattern=/user/login")},
		},
	}
	require.Nil(t, rule.Valid())

	rule = Rule{
		Action: RuleActionForward,
		Conditions: [][]Condition{
			{Condition("not-a-key=127.0.0.1")},
			{Condition("path-pattern=/user/login")},
		},
	}
	require.NotNil(t, rule.Valid())

	rule = Rule{
		Action: RuleActionForward,
		Conditions: [][]Condition{
			{Condition("source-ip=127.0.0.1")},
			{Condition("path-pattern not_a_op /user/login")},
		},
	}
	require.NotNil(t, rule.Valid())
}

func TestRuleMatches(t *testing.T) {
	sourceIp := "127.0.0.1"
	invalidSourceIp := "10.0.0.1"
	pathPattern := "/user/login"
	invalidPathPattern := "/not/the/path"
	httpMethod := http.MethodGet
	invalidHttpMethod := http.MethodPost
	hostHeader := "notexample.com"
	invalidHostHeader := "example.com"

	rule := Rule{
		Action: RuleActionForward,
		Conditions: [][]Condition{
			{Condition(fmt.Sprintf("source-ip=%s", sourceIp))},
			{Condition(fmt.Sprintf("path-pattern=%s",
				pathPattern))},
			{Condition(fmt.Sprintf("http-request-method=%s",
				httpMethod))},
			{Condition(fmt.Sprintf("host-header != %s",
				invalidHostHeader))},
		},
	}
	req, err := http.NewRequest(httpMethod, pathPattern, nil)
	require.Nil(t, err)
	req.Header.Set("Host", hostHeader)
	req.RemoteAddr = net.JoinHostPort(sourceIp, "8080")
	require.True(t, rule.Matches(req))

	req.RemoteAddr = net.JoinHostPort(invalidSourceIp, "8080")
	require.False(t, rule.Matches(req))

	req.RemoteAddr = net.JoinHostPort(sourceIp, "8080")
	req.Method = invalidHttpMethod
	require.False(t, rule.Matches(req))

	req.Method = httpMethod
	req.URL.Path = invalidPathPattern
	require.False(t, rule.Matches(req))

	req.URL.Path = pathPattern
	req.Header.Set("Host", invalidHostHeader)
	require.False(t, rule.Matches(req))
}

func TestRuleMatchesCIDR(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)

	// Match when source IPs are contained in CIDR range
	cond := Condition("source-ip = 127.0.0.0/24")
	req.RemoteAddr = net.JoinHostPort("127.0.0.10", "8080")
	rule := Rule{
		Action:     RuleActionForward,
		Conditions: [][]Condition{{cond}},
	}
	require.True(t, rule.Matches(req))
	req.RemoteAddr = net.JoinHostPort("127.0.2.10", "8080")
	require.False(t, rule.Matches(req))

	// Match when source IPs are NOT contained in CIDR range
	cond = Condition("source-ip != 127.0.0.0/24")
	rule.Conditions = [][]Condition{{cond}}
	require.True(t, rule.Matches(req))
	req.RemoteAddr = net.JoinHostPort("127.0.0.10", "8080")
	require.False(t, rule.Matches(req))
}
