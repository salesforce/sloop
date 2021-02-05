package webserver

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRedirectHandlerHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	assert.Nil(t, err)

	// Create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(RedirectHandler("clusterContext"))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTemporaryRedirect, rr.Code)
	assert.Equal(t, "/clusterContext", rr.Result().Header["Location"][0])
}

func TestWebFileHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/clusterContext/webfiles/index.html", nil)
	assert.Nil(t, err)

	// Create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(WebFileHandler("clusterContext"))
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, rr.Body.String())
}
