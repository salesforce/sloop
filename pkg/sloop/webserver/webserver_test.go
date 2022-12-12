package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectHandlerHandler(t *testing.T) {
	testCases := map[string]struct {
		url      string
		code     int
		location string
	}{
		"successfully redirect on /": {
			"/",
			http.StatusTemporaryRedirect,
			"/clusterContext",
		},
		"return 404 for invalid url": {
			"/an-invalid-url",
			http.StatusNotFound,
			"",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tc.url, nil)
			assert.Nil(t, err)
			// Create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(redirectHandler("clusterContext"))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.code, rr.Code)
			if len(tc.location) > 0 {
				assert.Equal(t, tc.location, rr.Result().Header["Location"][0])
			}
		})
	}
}

func TestWebFileHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/clusterContext/webfiles/index.html", nil)
	assert.Nil(t, err)

	// Create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(webFileHandler("clusterContext"))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, rr.Body.String())
}
