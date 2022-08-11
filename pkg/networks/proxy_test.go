package networks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReverseNetworkProxyProxy(t *testing.T) {
	body := "{\"hello\": \"world\"}"
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", body)
		}),
	)
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.Nil(t, err)
	rproxy := NewReverseNetworkProxy("tcp", targetUrl.Host, 3*time.Second)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	defer l.Close()
	go func() {
		conn, _ := l.Accept()
		ctx := context.Background()
		rproxy.Proxy(ctx, conn)
	}()

	resp, err := http.Get("http://" + l.Addr().String())
	require.Nil(t, err)
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, body, string(respBody))
}
