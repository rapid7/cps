package health

import (
	"encoding/json"
	"net/http"

	consul "cps/watchers/v1/consul"
	s3 "cps/watchers/v1/s3"

	log "github.com/sirupsen/logrus"
)

type Health struct {
	Status  int           `json:"status"`
	Plugins HealthPlugins `json:"plugins"`
}

type HealthPlugins struct {
	Consul bool `json:"consul"`
	S3     bool `json:"s3"`
}

func GetHealth(w http.ResponseWriter, r *http.Request) {
	var status int
	if s3.Health == true && consul.Health == true {
		status = 200
	} else {
		status = 503
	}

	data, err := json.Marshal(Health{
		Status: status,
		Plugins: HealthPlugins{
			Consul: consul.Health,
			S3:     s3.Health,
		},
	})
	if err != nil {
		log.Error(err)
		return
	}

	if status == 503 {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
