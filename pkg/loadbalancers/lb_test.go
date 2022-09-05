package loadbalancers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crossedbot/simpleloadbalancer/pkg/targets"
	"github.com/crossedbot/simpleloadbalancer/pkg/templates"
)

func TestHandleForbidden(t *testing.T) {
	rr1 := httptest.NewRecorder()
	errFmt := targets.ResponseFormatHtml
	expected := templates.ForbiddenPage()
	handleForbidden(rr1, errFmt)
	resp := rr1.Result()
	actual, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	expected = "Forbidden\n"
	rr2 := httptest.NewRecorder()
	errFmt = targets.ResponseFormatJson
	b, err := json.Marshal(targets.ResponseError{
		Code:    http.StatusForbidden,
		Message: expected[:len(expected)-1],
	})
	require.Nil(t, err)
	handleForbidden(rr2, errFmt)
	resp = rr2.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, b, actual)

	rr3 := httptest.NewRecorder()
	errFmt = targets.ResponseFormatPlain
	handleForbidden(rr3, errFmt)
	resp = rr3.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, expected, string(actual))

	rr4 := httptest.NewRecorder()
	errFmt = targets.ResponseFormatUnknown
	handleForbidden(rr4, errFmt)
	resp = rr4.Result()
	actual, err = ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Equal(t, expected, string(actual))
}
