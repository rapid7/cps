package health

import (
	"encoding/json"
	"net/http"

	consul "cps/watchers/v1/consul"
	s3 "cps/watchers/v1/s3"

	log "github.com/sirupsen/logrus"
)

type Response struct {
	Status string `json:"status"`
	Consul bool   `json:"consul"`
	S3     bool   `json:"s3"`
}

func GetHealthz(w http.ResponseWriter, r *http.Request) {
	var status string
	if s3.Up == true && consul.Up == true {
		status = "up"
	} else {
		status = "down"
	}

	data, err := json.Marshal(Response{
		Status: status,
		Consul: consul.Up,
		S3:     s3.Up,
	})
	if err != nil {
		log.Error(err)
	}

	if status == "down" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
