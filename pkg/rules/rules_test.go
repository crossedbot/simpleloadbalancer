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

func TestMatchRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	require.Nil(t, err)

	cond := Condition("host-header = example.com")
	req.Host = "example.com"
	require.True(t, matchRequest(cond, req))
	req.Host = "notexample.com"
	require.False(t, matchRequest(cond, req))
	cond = Condition("host-header != example.com")
	require.True(t, matchRequest(cond, req))
	req.Host = "example.com"
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
	req.Host = hostHeader
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
	req.Host = invalidHostHeader
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
