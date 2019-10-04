package consul

import (
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"

	"github.com/rapid7/cps/pkg/kv"
)

var (
	// Up is a measure of cps's readiness. If true there are no issues with s3.
	Up bool

	// Health is a measure of cps's overall health. If false the watcher
	// it is likely that the watcher could not list objects. In this case, there
	// are still probably objects in the kv store so the service is still
	// considered "Up".
	Health bool

	// Config contains minimal configuration information. Need to export
	// the config struct itself (TODO).
	Config       config
	healthyNodes map[string][]string
)

type config struct {
	host string
}

func init() {
	Health = false
	Up = false
}

// Poll polls every 60 seconds, kicking off a consul sync.
func Poll(host string, log *zap.Logger) {
	Config = config{
		host: host,
	}

	Sync(time.Now(), log)

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now(), log)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

// Sync connects to consul and gets a list of services and their health.
// Finally, it puts all healthy services into the kv store.
func Sync(t time.Time, log *zap.Logger) {
	log.Info("Consul sync begun")

	consulHost := Config.host
	client, err := setUpConsulClient(consulHost, log)
	if err != nil {
		return
	}

	services, qo, err := getServices(client, log)
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(services))

	numCores := runtime.NumCPU()
	guard := make(chan struct{}, numCores*16)

	healthyNodes = make(map[string][]string)

	var mutex = &sync.Mutex{}

	for key := range services {
		guard <- struct{}{}
		go func(key string) {
			defer wg.Done()
			getServiceHealth(key, client, qo, mutex, log)
			<-guard
		}(key)
	}

	wg.Wait()

	writeProperties()

	Health = true
	Up = true

	log.Info("Consul sync is finished")
}

func setUpConsulClient(consulHost string, log *zap.Logger) (*api.Client, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = consulHost
	consulConfig.Scheme = "http"

	client, err := api.NewClient(consulConfig)
	if err != nil {
		log.Error("Consul error",
			zap.Error(err),
			zap.String("consul_host", consulHost),
		)

		return nil, err
	}

	return client, nil
}

func getServices(client *api.Client, log *zap.Logger) (map[string][]string, api.QueryOptions, error) {
	catalog := client.Catalog()
	qo := api.QueryOptions{}
	services, _, err := catalog.Services(&qo)
	if err != nil {
		log.Error("Consul Catalog/services error failed",
			zap.Error(err),
		)

		return nil, qo, err
	}

	return services, qo, nil
}

func writeProperties() {
	kv.WriteProperty("consul", healthyNodes)
}

func getServiceHealth(key string, client *api.Client, qo api.QueryOptions, m *sync.Mutex, log *zap.Logger) {
	h := client.Health()
	sh, _, err := h.Service(key, "", true, &qo)
	if err != nil {
		log.Error("Failed to find service",
			zap.Error(err),
			zap.String("service", key),
		)

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
			log.Info("Skipping service!",
				zap.String("service", key),
				zap.String("aggregated_status", as),
			)
		}
	}
}
