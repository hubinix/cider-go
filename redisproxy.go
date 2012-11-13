package main

import (
	"./rediscluster"
	"flag"
	"hash/adler32"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"fmt"
	"os"
	"runtime/pprof"
	"strings"
)

type RedisServerList []*rediscluster.RedisShardGroup

func (rsl *RedisServerList) Set(s string) error {
	for _, group := range strings.Split(s, ";") {
		log.Printf("Creating shard group")
		shardGroup := rediscluster.RedisShardGroup{}
		for _, shard := range strings.Split(group, ",") {
			parts := strings.Split(shard, ":")
			if len(parts) != 3 {
				return fmt.Errorf("Invalid shard format, must be in host:port:db form: %s", shard)
			}

			id := int(adler32.Checksum([]byte(shard)))
			host := parts[0]
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return fmt.Errorf("Could not parse port for shard: %s", shard)
			}
			db, err := strconv.Atoi(parts[2])
			if err != nil {
				return fmt.Errorf("Could not parse db for shard: %s", shard)
			}

			s := rediscluster.NewRedisShard(id, host, port, db)
			if s == nil {
				return fmt.Errorf("Could not create shard (probably a connection problem): %s", shard)
			}

			log.Printf("[%d] Added shard: %s", id, shard)
			shardGroup.AddShard(s)
		}
		shardGroup.Start()
		*rsl = append(*rsl, &shardGroup)
	}
	return nil
}

func (rsp *RedisServerList) String() string {
	return "RedisServerList"
}

var NetAddr string
var RedisServers RedisServerList
var redisCluster *rediscluster.RedisCluster

var (
	Clients = uint64(0)
	OK      = rediscluster.MessageFromString("+OK\r\n")
)

type RedisClient struct {
	Conn           net.Conn
	NumRequests    uint64
	ConnectionTime time.Time
	*rediscluster.RedisProtocol
}

func NewRedisClient(conn net.Conn) *RedisClient {
	client := RedisClient{
		Conn:           conn,
		RedisProtocol:  rediscluster.NewRedisProtocol(conn),
		NumRequests:    0,
		ConnectionTime: time.Now(),
	}
	return &client
}

func (rc *RedisClient) Handle() error {
	var err error

	f, err := os.Create(fmt.Sprintf("%s.pprof", rc.Conn.RemoteAddr()))
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	isPipeline := false
	var pipeline *rediscluster.RedisClusterPipeline
	for {
		request, err := rc.ReadMessage()
		if err != nil {
			return err
		}
		atomic.AddUint64(&rc.NumRequests, 1)
		command := request.Command()
		var response *rediscluster.RedisMessage
		if command == "MULTI" {
			isPipeline = true
			pipeline = rediscluster.NewRedisClusterPipeline(redisCluster)
			response = OK
		} else {
			if command == "EXEC" {
				isPipeline = false
				response := pipeline.Execute()
				rc.WriteMessage(response)
			} else {
				if isPipeline {
					response, err = pipeline.Send(request)
					if err != nil {
						log.Printf("Error getting response: %s", err)
						return err
					}
				} else {
					response, err = redisCluster.Do(request)
					if err != nil {
						log.Printf("Error getting response: %s", err)
						return err
					}
				}
			}
		}
		rc.WriteMessage(response)
	}
	return err
}

func main() {
	flag.StringVar(&NetAddr, "net-address", ":6543", "Net address that the redis proxy will listen on")
	flag.Var(&RedisServers, "redis-group", "List of redis shards that form one redundant redis shard-group (may be given multiple times to specify multiple shard-groups)")
	flag.Parse()

	if len(RedisServers) == 0 {
		fmt.Println("Missing argument: redis-cluster")
		flag.Usage()
		return
	}

	listener, err := net.Listen("tcp", NetAddr)
	if err != nil {
		log.Fatalf("Could not bind to address %s", NetAddr)
	}

	redisCluster = rediscluster.NewRedisCluster(RedisServers...)
	log.Printf("Started redis cluster")

	log.Printf("Listening to connections on %s", NetAddr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			// handle error
			continue
		}
		client := NewRedisClient(conn)
		go func(client *RedisClient) {
			Clients += 1
			log.Printf("Got connection from: %s. %d clients connected", client.Conn.RemoteAddr(), Clients)
			client.Handle()
			Clients -= 1
			log.Printf("Client disconnected: %s (after %d requests). %d clients connected", client.Conn.RemoteAddr(), client.NumRequests, Clients)
		}(client)
	}
}