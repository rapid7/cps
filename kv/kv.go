package kv

import (
	"sync"
)

/**
*
* global map
*
**/
var (
	// Cache is a global map that any package can use to get properties.
	Cache = sync.Map{}
)

// WriteProperty writes a property to the Cache.
func WriteProperty(k string, v interface{}) error {
	Cache.Store(k, v)
	return nil
}

// DeleteProperty deletes a property from the Cache.
func DeleteProperty(k interface{}) error {
	Cache.Delete(k)
	return nil
}

// GetProperty gets a property from the Cache.
func GetProperty(k interface{}) interface{} {
	v, _ := Cache.Load(k)
	return v
}
