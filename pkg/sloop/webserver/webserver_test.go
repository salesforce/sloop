package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	badger "github.com/dgraph-io/badger/v2"
	"github.com/salesforce/sloop/pkg/sloop/store/untyped/badgerwrap"
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

func TestBackupHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/clusterContext/data/backup", nil)
	assert.Nil(t, err)

	db, err := (&badgerwrap.MockFactory{}).Open(badger.DefaultOptions(""))
	assert.Nil(t, err)

	// Create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(backupHandler(db, "clusterContext"))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, rr.Body.String())
}
