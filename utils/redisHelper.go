package utils

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
)

var mutex sync.Mutex

// remove AllowedPaths:Role:id
func ClearPathsCache(roleId int) error {
	return config.RemoveRedisKey("AllowedPaths:Role:" + fmt.Sprint(roleId))
}

func GetCacheLifespan() time.Duration {
	lifespan, err := strconv.Atoi(os.Getenv("CACHE_LIFESPAN"))
	if err != nil {
		lifespan = 1
	}
	return time.Duration(lifespan) * time.Hour
}

/* generic functions */

func GetTypeName[T any]() string {
	var v T
	typeOfT := reflect.TypeOf(v)
	return typeOfT.Name()
}

// get type name of struct
func GetType(i interface{}) string {
	return reflect.TypeOf(i).Name()
}

/* Redis */

// check if model has expiration date
func typeHasExpiration(typeName string) bool {
	expirableTypes := map[string]bool{
		"Product":         true,
		"ProductCategory": true,
		"ProductModifier": true,
		"ProductUnit":     true,
		"ProductVariant":  true,
		"ProductGroup":    true,
	}
	return expirableTypes[typeName]
}

// store instance, obj should be a pointer
func StoreRedis[T any](obj any, id int) error {
	typeName := GetTypeName[T]()
	key := typeName + ":" + fmt.Sprint(id)

	var duration time.Duration
	if typeHasExpiration(typeName) {
		duration = GetCacheLifespan()
	}
	return config.SetRedisObject(key, &obj, duration)
}

// // store instance, obj should be a pointer
// func StoreRedisExp[T any](obj any, id int) error {
// 	key := getTypeName[T]() + ":" + fmt.Sprint(id)
// 	return config.SetRedisObject(key, &obj, GetCacheLifespan())
// // }

// store object
func StoreRedisList[T any](obj any, businessId string) error {
	var key string
	typeName := GetTypeName[T]()
	if businessId == "" {
		key = GetTypeName[T]() + "List"
	} else {
		key = GetTypeName[T]() + "List:" + businessId
	}

	var duration time.Duration
	if typeHasExpiration(typeName) {
		duration = GetCacheLifespan()
	}
	return config.SetRedisObject(key, &obj, duration)
}

// get from redis
// returns nil if does not exist
func RetrieveRedis[T any](id int) (*T, error) {
	var result *T
	key := GetTypeName[T]() + ":" + fmt.Sprint(id)
	exists, err := config.GetRedisObject(key, &result)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return result, nil
}

// retrieve a list.
// businessId can be empty
func RetrieveRedisList[T any](businessId string) ([]*T, error) {
	var key string
	if businessId == "" {
		key = GetTypeName[T]() + "List"
	} else {
		key = GetTypeName[T]() + "List:" + businessId
	}

	var result []*T
	exists, err := config.GetRedisObject(key, &result)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return result, nil
}

// clear list, TypeList:$business_id
func RemoveRedisList[T any](businessId string) error {
	var key string = GetTypeName[T]() + "List:" + businessId
	return config.RemoveRedisKey(key)
}

func RemoveRedisMap[T any](businessId string) error {
	var key string = GetTypeName[T]() + "Map:" + businessId
	return config.RemoveRedisKey(key)
}

// remove an instance, Type:$id
func RemoveRedisItem[T any](id int) error {
	key := GetTypeName[T]() + ":" + fmt.Sprint(id)
	return config.RemoveRedisKey(key)
}

func ClearRedisAdmin[T any]() error {
	if err := config.RemoveRedisKey("All" + GetTypeName[T]() + "List"); err != nil {
		return err
	}
	if err := config.RemoveRedisKey("All" + GetTypeName[T]() + "Map"); err != nil {
		return err
	}
	return nil
}

func GetSequence[T any](ctx context.Context, businessId string) (int64, error) {
	// lock
	var model T
	// fmt.Println("waiting for mutex: " + time.Now().Format("01:04:05"))
	mutex.Lock()
	defer mutex.Unlock()
	// fmt.Println("UNLOCked mutex: " + time.Now().Format("01:04:05"))
	cacheKey := businessId + "-" + strings.ToLower(GetTypeName[T]()) + "_seq"
	var seqNo int64
	var err error
	db := config.GetDB()

	for {
		seqNo, err = config.GetRedisCounter(ctx, cacheKey)
		if err != nil {
			return 0, err
		}
		// if not found in redis, get from db
		if seqNo == 1 {
			// get max seq no from db
			var dbSeq *int64
			if err := db.WithContext(ctx).Model(&model).Select("max(sequence_no)").
				Where("business_id = ?", businessId).
				Scan(&dbSeq).Error; err != nil {
				return 0, err
			}
			// in case db has no journal records
			if dbSeq == nil {
				seqNo = 0
			} else {
				seqNo = *dbSeq
			}
			// set redis
			seqNo++
			if err := config.SetRedisObject(cacheKey, &seqNo, 0); err != nil {
				return 0, err
			}
		}
		// check if sequence number exists in db
		err = ValidateUnique[T](ctx, businessId, "sequence_no", seqNo, 0)
		if err == nil {
			break
		}
	}
	// time.Sleep(time.Second * 5)
	// unlock
	return seqNo, nil
}
