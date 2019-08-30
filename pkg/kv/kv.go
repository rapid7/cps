package kv

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

/**
*
* global map
*
**/
var (
	// Global map that any package can use to get properties.
	Cache = sync.Map{}
)

// Writes a property to the Cache.
func WriteProperty(k string, v interface{}) error {
	Cache.Store(k, v)
	return nil
}

// Deletes a property from the Cache.
func DeleteProperty(k interface{}) error {
	Cache.Delete(k)
	log.Printf("Deleted key: %v", k)
	return nil
}

// Gets a property from the Cache.
func GetProperty(k interface{}) interface{} {
	v, _ := Cache.Load(k)
	return v
}
