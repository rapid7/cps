package properties

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/logger"
)

var (
	account string
	region  string
)

func TestGetProperties(t *testing.T) {
	account = "123456"
	region = "us-east-1"
	service := "service-one"
	path := account + "/" + region + "/" + service

	serviceOneProperties := map[string]interface{}{
		"string-prop": "string",
		"bool-prop":   true,
		"int-prop":    1,
		"float-prop":  1.5,
	}

	kv.WriteProperty(path, serviceOneProperties)
	kv.WriteProperty("consul", map[string][]string{"service-one": {"127.0.0.1"}})

	req, err := http.NewRequest("GET", "/v1/conqueso/service-one", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"service": "service-one"})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(toHandleAllProps)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code is wrong: expected %v got %v", http.StatusOK, status)
	}

	expectedJSON := `{"bool-prop":true,"consul":{"service-one":["127.0.0.1"]},"float-prop":1.5,"int-prop":1,"string-prop":"string"}`
	assert.Equal(t, expectedJSON, rr.Body.String())

}

func toHandleAllProps(w http.ResponseWriter, r *http.Request) {
	log := logger.BuildLogger()

	GetProperties(w, r, account, region, log)
}

func TestGetProperty(t *testing.T) {
	account = "123456"
	region = "us-east-1"
	service := "service-one"
	path := account + "/" + region + "/" + service

	serviceOneProperties := map[string]interface{}{
		"string-prop": "string",
		"bool-prop":   true,
		"int-prop":    1,
		"float-prop":  1.5,
	}

	kv.WriteProperty(path, serviceOneProperties)
	kv.WriteProperty("consul", map[string][]string{"service-one": {"127.0.0.1"}})

	req, err := http.NewRequest("GET", "/v1/conqueso/service-one/string-prop", nil)
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"service": "service-one", "property": "string-prop"})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(toHandleOneProp)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code is wrong: expected %v got %v", http.StatusOK, status)
	}

	expectedOutput := `string`
	assert.Equal(t, expectedOutput, rr.Body.String())
}

func toHandleOneProp(w http.ResponseWriter, r *http.Request) {
	log := logger.BuildLogger()

	GetProperty(w, r, account, region, log)
}
