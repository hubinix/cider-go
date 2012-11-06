package rediscluster

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"log"
)

const (
	REDIS_DISCONNECTED = iota
	REDIS_CONNECTED
	REDIS_READONLY
	REDIS_WRITEONLY
)

type RedisShard struct {
	Id        int
	Host      string
	Port      int
	Db        int
	Status    int
	LastError error

	rdb redis.Conn
}

func (rs *RedisShard) Connect() int {
	var err error
	rs.rdb, err = redis.Dial("tcp", fmt.Sprintf("%s:%d", rs.Host, rs.Port))
	if err == nil {
		status, err := redis.String(rs.rdb.Do("SELECT", rs.Db))
		if status != "OK" || err != nil {
			log.Printf("[shard %d] Could not change to DB %d", rs.Id, rs.Db)
			rs.LastError = err
		} else {
			rs.Status = REDIS_CONNECTED
			log.Printf("[shard %d] Connected on %s:%d:%d", rs.Id, rs.Host, rs.Port, rs.Db)
		}
	} else {
		log.Printf("[shard %d] Could not connect: %s", rs.Id, err)
		rs.LastError = err
	}
	return rs.Status
}

func (rs *RedisShard) Close() {
	rs.rdb.Close()
}
