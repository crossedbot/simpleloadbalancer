package targets

import (
	"strings"
)

// ResponseError represents a response error structure.
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ResponseFormat represents a target response format.
type ResponseFormat uint32

const (
	// Formats
	ResponseFormatUnknown ResponseFormat = iota
	ResponseFormatHtml
	ResponseFormatJson
	ResponseFormatPlain
)

const DefaultResponseFormat = ResponseFormatPlain

// ResponseFormatStrings is a list of string representations of known response
// formats.
var ResponseFormatStrings = []string{
	"unknown",
	"html",
	"json",
	"plain",
}

// ToResponseFormat returns the ResponseFormat for a given string. If a match
// can not be made, ResponseFormatUnknown is returned.
func ToResponseFormat(v string) ResponseFormat {
	for idx, s := range ResponseFormatStrings {
		if strings.EqualFold(s, v) {
			return ResponseFormat(idx)
		}
	}
	return ResponseFormatUnknown
}

// String returns the string representation for a given response format. If the
// response format is not known the string representation of
// RepsonseFormatUnknown is returned instead.
func (f ResponseFormat) String() string {
	if f > ResponseFormat(len(ResponseFormatStrings)) {
		f = ResponseFormatUnknown
	}
	return ResponseFormatStrings[int(f)]
}
