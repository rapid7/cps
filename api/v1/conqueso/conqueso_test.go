package conqueso

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"

	"github.com/rapid7/cps/pkg/kv"
)

var (
	account string
	region  string
)

func TestGetConquesoProperties(t *testing.T) {
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
	handler := http.HandlerFunc(toHandle)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Status code is wrong: expected %v got %v", http.StatusOK, status)
	}

	assert.Contains(t, rr.Body.String(), "string-prop=string")
	assert.Contains(t, rr.Body.String(), "bool-prop=true")
	assert.Contains(t, rr.Body.String(), "int-prop=1")
	assert.Contains(t, rr.Body.String(), "float-prop=1.5")
}

func toHandle(w http.ResponseWriter, r *http.Request) {
	GetConquesoProperties(w, r, account, region)
}
