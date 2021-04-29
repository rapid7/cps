package conqueso

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/rapid7/cps/kv"
)

// GetConquesoProperties is a Handler for /v1/conqueso/{service}.
// It returns a service's properties in the java property style.
func GetConquesoProperties(w http.ResponseWriter, r *http.Request, account string, region string, log *zap.Logger) {
	vars := mux.Vars(r)
	service := vars["service"]

	var path bytes.Buffer
	path.WriteString(account)
	path.WriteString("/")
	path.WriteString(region)
	path.WriteString("/")
	path.WriteString(service)

	serviceProperties := kv.GetProperty(path.String()).(map[string]interface{})

	var output bytes.Buffer
	consulProperties := kv.GetProperty("consul").(map[string][]string)
	for k, v := range consulProperties {
		key := "conqueso." + k + ".ips="
		output.WriteString(key)
		for i, ip := range v {
			if len(v) == i+1 {
				output.WriteString(ip)
			} else {
				output.WriteString(ip + ",")
			}
		}
		output.WriteString("\n")
	}

	for k, v := range serviceProperties {
		var line string
		switch t := v.(type) {
		case string:
			line = k + "=" + v.(string) + "\n"
		case int:
			line = k + "=" + strconv.Itoa(v.(int)) + "\n"
		case bool:
			line = k + "=" + strconv.FormatBool(v.(bool)) + "\n"
		case float64:
			line = k + "=" + strconv.FormatFloat(v.(float64), 'f', -1, 64) + "\n"
		default:
			log.Error("Unsupported type!",
				zap.String("key", k),
				zap.Any("value", v),
				zap.Any("type", t),
			)
		}
		output.WriteString(line)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(output.Bytes())
}

// PostConqueso is an empty handler constructed to deal with a bug in the java
// conqueso client. It just needs to accept a POST, it does not need to return
// anything.
func PostConqueso(w http.ResponseWriter, r *http.Request) {
}
