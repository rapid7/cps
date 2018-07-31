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
	Cache = sync.Map{}
)

func WriteProperty(k string, v interface{}) error {
	Cache.Store(k, v)
	return nil
}

func DeleteProperty(k interface{}) error {
	Cache.Delete(k)
	log.Printf("Deleted key: %v", k)
	return nil
}

func GetProperty(k interface{}) interface{} {
	v, _ := Cache.Load(k)
	return v
}
