package redis

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis"
)

type redisStruct struct {
	client *redis.Client
}

type RedisInterface interface {
	Connect(host string, db int) error
	GetHashKeys(hash string) ([]string, error)
	GetHashValues(hash, key string) string
	SetHash(hash, key, value string) error
	Close()
}

func New() RedisInterface {
	return &redisStruct{}
}

func (r *redisStruct) Connect(host string, db int) error {
	r.client = redis.NewClient(&redis.Options{
		Addr: host,
		DB:   db,
	})

	_, err := r.client.Ping().Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *redisStruct) GetHashKeys(hash string) ([]string, error) {
	exist := r.client.Exists(hash).Val()
	if exist != 1 {
		return make([]string, 0), errors.New(fmt.Sprintf("hash %s can not found in production redis", hash))
	}
	return r.client.HKeys(hash).Val(), nil
}

func (r *redisStruct) GetHashValues(hash, key string) string {
	keyValue := r.client.HGet(hash, key).Val()
	return keyValue
}

func (r *redisStruct) SetHash(hash, key, value string) error {
	setHashError := r.client.HSet(hash, key, value).Err()
	return setHashError
}

func (r *redisStruct) Close() {
	_ = r.client.Close()
}
