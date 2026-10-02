package faker

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestHTTPRemotePinsURLBoundsTimeAndResponse(t *testing.T) {
	for _, test := range []struct {
		label     string
		status    int
		body      string
		wantError bool
	}{
		{"source", http.StatusOK, "export default ['qzx'];", false},
		{"missing", http.StatusNotFound, "missing", true},
		{"oversized", http.StatusOK, strings.Repeat("x", maxSourceBytes+1), true},
	} {
		t.Run(test.label, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://raw.githubusercontent.com/faker-js/faker/"+Revision+"/src/file.ts" {
					t.Fatalf("unpinned URL: %s", req.URL)
				}
				if _, ok := req.Context().Deadline(); !ok {
					t.Fatal("request has no deadline")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})}
			data, err := HTTPRemote(client)(context.Background(), "src/file.ts")
			if (err != nil) != test.wantError {
				t.Fatalf("response error=%v", err)
			}
			if !test.wantError && string(data) != test.body {
				t.Fatal("response changed")
			}
		})
	}
}
