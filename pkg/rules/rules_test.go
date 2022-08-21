package rules

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleValid(t *testing.T) {
	rule := Rule{
		Action: RuleActionForward,
		Conditions: []Condition{
			Condition("source-ip=127.0.0.1"),
			Condition("path-pattern=/user/login"),
		},
	}
	require.Nil(t, rule.Valid())

	rule = Rule{
		Action: RuleActionForward,
		Conditions: []Condition{
			Condition("not-a-key=127.0.0.1"),
			Condition("path-pattern=/user/login"),
		},
	}
	require.NotNil(t, rule.Valid())

	rule = Rule{
		Action: RuleActionForward,
		Conditions: []Condition{
			Condition("source-ip=127.0.0.1"),
			Condition("path-pattern not_a_op /user/login"),
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
		Conditions: []Condition{
			Condition(fmt.Sprintf("source-ip=%s", sourceIp)),
			Condition(fmt.Sprintf("path-pattern=%s", pathPattern)),
			Condition(fmt.Sprintf("http-request-method=%s",
				httpMethod)),
			Condition(fmt.Sprintf("host-header != %s",
				invalidHostHeader)),
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
