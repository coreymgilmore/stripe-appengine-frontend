/*
Package memcachutils implements some wrapper functions around the appengine memcache library to make using
memcache easier and to reduce the amount of retyped code.
*/

package memcacheutils

import (
	"golang.org/x/net/context"

	"google.golang.org/appengine/memcache"
)

//Save saves a key value pair to memcache
//the key is usually a datastore intId represented as a string
//this saves an object as the value. the value isn't just a string
func Save(c context.Context, key string, value interface{}) error {
	//build memcache item to store
	item := &memcache.Item{
		Key:    key,
		Object: value,
	}

	//save
	err := memcache.Gob.Set(c, item)
	if err != nil {
		return err
	}

	//done
	return nil
}

//Delete removes a key value pair from memcache
func Delete(c context.Context, key string) error {
	err := memcache.Delete(c, key)
	if err == memcache.ErrCacheMiss {
		//key does not exist
		//this is not an error
		return nil
	} else if err != nil {
		return err
	}

	//delete successful
	return nil
}
