package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rapid7/cps/logger"
	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/s3"
)

func TestGetHealth(t *testing.T) {
	log := logger.BuildLogger()

	req, err := http.NewRequest("GET", "/v1/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	consulEnabled := false
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GetHealth(w, r, log, consulEnabled)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("Status code is wrong when unhealthy: expected %v got %v", status, http.StatusServiceUnavailable)
	}

	consulEnabled = true
	consul.Health = true
	s3.Health = true

	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		GetHealth(w, r, log, consulEnabled)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code is wrong when services are healthy: expected %v got %v", status, http.StatusOK)
	}

	assert.NotNil(t, rr.Body.String())

	expectedJSON := `{"status":200,"plugins":{"consul":true,"s3":true}}`
	assert.Equal(t, expectedJSON, rr.Body.String())
}
