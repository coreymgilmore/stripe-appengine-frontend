package memcacheutils 

import(
	"appengine"
	"appengine/memcache"
)


//SAVE TO MEMCACHE
//key is actually an int as a string (the intID of a key)
func Save(c appengine.Context, key string, value interface{}) error {
	//build memcache item to store
	item := &memcache.Item{
		Key: 	key,
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

//DELETE FROM MEMCACHE
func Delete(c appengine.Context, key string) error {
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
