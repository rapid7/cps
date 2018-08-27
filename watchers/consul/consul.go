package consul

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	log "github.com/sirupsen/logrus"

	kv "cps/pkg/kv"
)

var (
	Up           bool
	Health       bool
	Config       config
	healthyNodes map[string][]string
)

type config struct {
	host string
}

func init() {
	Health = false
	Up = false

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.Print("connecting to consul...")
}

func Poll(host string) {
	Config = config{
		host: host,
	}

	Sync(time.Now())

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now())
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func Sync(t time.Time) {
	log.Print("consul sync begun")

	consulHost := Config.host
	client, err := setUpConsulClient(consulHost)
	if err != nil {
		return
	}

	services, qo, err := getServices(client)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(services))

	numCores := runtime.NumCPU()
	guard := make(chan struct{}, numCores*16)

	healthyNodes = make(map[string][]string)

	var mutex = &sync.Mutex{}

	for key, _ := range services {
		guard <- struct{}{}
		go func(key string) {
			defer wg.Done()
			getServiceHealth(key, client, qo, mutex)
			<-guard
		}(key)
	}

	wg.Wait()

	writeProperties()

	Health = true
	Up = true

	log.Print("Consul sync is finished")
}

func setUpConsulClient(consulHost string) (*api.Client, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = consulHost
	consulConfig.Scheme = "http"

	client, err := api.NewClient(consulConfig)
	if err != nil {
		log.Errorf("Consul error: %v\n", err)
		return nil, err
	}

	return client, nil
}

func getServices(client *api.Client) (map[string][]string, api.QueryOptions, error) {
	catalog := client.Catalog()
	qo := api.QueryOptions{}
	services, _, err := catalog.Services(&qo)
	if err != nil {
		log.Errorf("Catalog/services error: %v", err)
		return nil, qo, err
	}

	return services, qo, nil
}

func writeProperties() {
	kv.WriteProperty("consul", healthyNodes)
}

func getServiceHealth(key string, client *api.Client, qo api.QueryOptions, m *sync.Mutex) {
	h := client.Health()
	sh, _, err := h.Service(key, "", true, &qo)
	if err != nil {
		log.Errorf("Failed to find service: %v", err)
		return
	}

	// Initialize an empty key in case no services are healthy.
	var emptyIp []string
	m.Lock()
	healthyNodes[key] = emptyIp
	m.Unlock()

	for _, element := range sh {
		as := element.Checks.AggregatedStatus()
		ip := element.Node.Address
		// log.Printf("Service health for %v is %v on %v", key, as, ip)
		if as == "passing" {
			m.Lock()
			healthyNodes[key] = append(healthyNodes[key], ip)
			m.Unlock()
		} else {
			log.Printf("Service %v is %v skipping!", key, as)
		}
	}
}
