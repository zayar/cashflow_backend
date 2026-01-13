package config

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/bsm/redislock"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var (
	rdb    *redis.Client
	locker *redislock.Client
)
var ctx = context.Background()

func GetRedisDB() *redis.Client {
	return rdb
}

func GetRedisLock() *redislock.Client {
	return locker
}

func GetRedisContext() context.Context {
	return ctx
}

func GetRedisObject(key string, dest interface{}) (bool, error) {
	if rdb == nil {
		return false, nil
	}
	// fmt.Printf("	(Redis) Getting object of `%s`\n", key)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return false, nil
		}
		return false, err
	}
	err = json.Unmarshal([]byte(val), &dest)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetRedisValue(key string) (string, bool, error) {
	if rdb == nil {
		return "", false, nil
	}
	// fmt.Printf("	(Redis) Getting value of `%s`\n", key)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return "", false, nil
		}
		return "", false, err
	}
	return val, true, nil
}

func SetRedisObject(key string, obj interface{}, exp time.Duration) error {
	if rdb == nil {
		return nil
	}
	// fmt.Printf("	(Redis) Setting object `%s`:%+v\n", key, obj)
	objInByte, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	if err = rdb.Set(ctx, key, objInByte, exp).Err(); err != nil {
		return err
	}
	return nil
}

// store key in a set for faster adding & retrieving
func AddRedisSet(setKey string, member string) error {
	if rdb == nil {
		return nil
	}
	if err := rdb.SAdd(ctx, setKey, member).Err(); err != nil {
		return err
	}
	return nil
}

func GetRedisSetMembers(setKey string) ([]string, error) {
	if rdb == nil {
		return nil, nil
	}
	return rdb.SMembers(ctx, setKey).Result()
}

func RemoveRedisSetMember(setKey string, member string) error {
	if rdb == nil {
		return nil
	}
	return rdb.SRem(ctx, setKey, member).Err()
}

func SetRedisValue(key string, value string, exp time.Duration) error {
	// fmt.Printf("	(Redis) Setting value `%s`:%s\n", key, value)
	// rdb.Set
	if rdb == nil {
		return nil
	}
	return rdb.Set(ctx, key, value, exp).Err()
}

func RemoveRedisKey(keys ...string) error {
	// fmt.Printf("	(Redis) Removing `%v`\n", keys)
	if rdb == nil {
		return nil
	}
	_, err := rdb.Del(ctx, keys...).Result()
	return err
}

func ClearRedis(ctx context.Context) error {
	if rdb == nil {
		return nil
	}
	cmd := rdb.FlushAll(ctx)
	return cmd.Err()
}

// add one and returns it, while storing the updated value
func GetRedisCounter(ctx context.Context, key string) (int64, error) {
	// exists, err := GetRedisObject(key, &value)
	// if err != nil {
	// 	return 0, err
	// }
	// if !exists {
	// 	// TODO: get from database
	// 	return 0, nil
	// }
	// result, err := rdb.Incr(ctx, key).Result()
	// if err != nil {
	// 	return 0, err
	// }

	if rdb == nil {
		return 0, nil
	}
	return rdb.Incr(ctx, key).Result()
}

func init() {
	// Load env from .env
	godotenv.Load()
	// IMPORTANT (Cloud Run):
	// Do NOT block startup in init() waiting for Redis.
	// Cloud Run requires the container to start listening on $PORT quickly.
}

// ConnectRedisWithRetry connects and sets the global Redis client + lock client.
// Call this from main() AFTER the HTTP server is listening.
func ConnectRedisWithRetry() {
	redisAddr := os.Getenv("REDIS_ADDRESS")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
		log.Printf("REDIS_ADDRESS not set; defaulting to %s", redisAddr)
	}

	var attempt int
	for {
		attempt++
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: "",
			DB:       0, // use default DB
			PoolSize: 100,
		})
		if err := rdb.Ping(ctx).Err(); err == nil {
			locker = redislock.New(rdb)
			log.Printf("connected to redis (attempt=%d addr=%s)", attempt, redisAddr)
			return
		} else {
			sleep := time.Second * time.Duration(1<<min(attempt, 5))
			if sleep > 30*time.Second {
				sleep = 30 * time.Second
			}
			log.Printf("failed to connect redis (attempt=%d addr=%s): %v; retrying in %s", attempt, redisAddr, err, sleep)
			time.Sleep(sleep)
		}
	}
}
