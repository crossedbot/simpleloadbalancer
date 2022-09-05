package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToResponseFormat(t *testing.T) {
	tests := []struct {
		Str      string
		Expected ResponseFormat
	}{
		{"unknown", ResponseFormatUnknown},
		{"hTmL", ResponseFormatHtml},
		{"JSON", ResponseFormatJson},
		{"plain", ResponseFormatPlain},
		{"wat", ResponseFormatUnknown},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected, ToResponseFormat(test.Str))
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		Fmt      ResponseFormat
		Expected string
	}{
		{ResponseFormatUnknown, "unknown"},
		{ResponseFormatHtml, "html"},
		{ResponseFormatJson, "json"},
		{ResponseFormatPlain, "plain"},
		{ResponseFormat(1000), "unknown"},
	}
	for _, test := range tests {
		require.Equal(t, test.Expected, test.Fmt.String())
	}

}
