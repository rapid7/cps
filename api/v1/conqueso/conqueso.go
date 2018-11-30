package conqueso

import (
	"bytes"
	"net/http"
	"os"
	"strconv"

	mux "github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/rapid7/cps/pkg/kv"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func GetConquesoProperties(w http.ResponseWriter, r *http.Request, account string, region string) {
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
			log.Errorf("Could not parse %v:%v, v is of type %T", k, v, t)
		}
		output.WriteString(line)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(output.Bytes())
}

func PostConqueso(w http.ResponseWriter, r *http.Request) {
}
