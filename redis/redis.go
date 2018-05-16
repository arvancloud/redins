package redis

import (
    "strings"
    "time"
    "log"
    "strconv"
    "errors"

    redisCon "github.com/garyburd/redigo/redis"
    "arvancloud/redins/config"
)

type Redis struct {
    Config       *config.RedisConfig
    Pool         *redisCon.Pool
    RedisAddress string
}

func NewRedis(config *config.RedisConfig) *Redis {
    r := &Redis {
        Config:         config,
        RedisAddress:   config.Ip + ":" + strconv.Itoa(config.Port),
    }

    r.Connect()
    return r
}

func (redis *Redis) Connect() {
    redis.Pool = &redisCon.Pool{
        Dial: func() (redisCon.Conn, error) {
            opts := []redisCon.DialOption{}
            if redis.Config.Password != "" {
                opts = append(opts, redisCon.DialPassword(redis.Config.Password))
            }
            if redis.Config.ConnectTimeout != 0 {
                opts = append(opts, redisCon.DialConnectTimeout(time.Duration(redis.Config.ConnectTimeout)* time.Millisecond))
            }
            if redis.Config.ReadTimeout != 0 {
                opts = append(opts, redisCon.DialReadTimeout(time.Duration(redis.Config.ReadTimeout)*time.Millisecond))
            }

            return redisCon.Dial("tcp", redis.RedisAddress, opts...)
        },
    }
}

func (redis *Redis) Get(key string) string {
    var (
        err   error
        reply interface{}
        val   string
    )
    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return ""
    }
    defer conn.Close()

    reply, err = conn.Do("GET", redis.Config.Prefix + key + redis.Config.Suffix)
    if err != nil {
        return ""
    }
    val, err = redisCon.String(reply, nil)
    if err != nil {
        return ""
    }
    return val
}

func (redis *Redis) Set(key string, value string) error {
    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return errors.New("connection to redis failed")
    }
    defer conn.Close()

    _, err := conn.Do("SET", redis.Config.Prefix + key + redis.Config.Suffix, value)
    if err != nil {
        log.Printf("[ERROR] redis error : %s", err)
        return err
    }
    return nil
}

func (redis *Redis) Del(pattern string) {
    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return
    }
    defer conn.Close()

    conn.Do("EVAL", "return redis.call('del', unpack(redis.call('keys', ARGV[1])))", 0, redis.Config.Prefix + pattern + redis.Config.Suffix)
}

func (redis *Redis) GetKeys() []string {
    var (
        reply interface{}
        err   error
        keys []string
    )

    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return nil
    }
    defer conn.Close()

    // TODO: use SCAN
    reply, err = conn.Do("KEYS", redis.Config.Prefix + "*" + redis.Config.Suffix)
    if err != nil {
        return nil
    }
    keys, err = redisCon.Strings(reply, nil)
    for i, _ := range keys {
        keys[i] = strings.TrimPrefix(keys[i], redis.Config.Prefix)
        keys[i] = strings.TrimSuffix(keys[i], redis.Config.Suffix)
    }
    return keys
}

func (redis *Redis) GetHKeys(key string) []string {
    var (
        reply interface{}
        err   error
        vals  []string
    )

    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return nil
    }
    defer conn.Close()

    reply, err = conn.Do("HKEYS", redis.Config.Prefix + key + redis.Config.Suffix)
    if err != nil {
        log.Printf("[ERROR] error in redis command : %s", err)
        return nil
    }
    vals, err = redisCon.Strings(reply, nil)
    if err != nil {
        return nil
    }
    return vals
}

func (redis *Redis) HGet(key string, hkey string) string {
    var (
        err   error
        reply interface{}
        val   string
    )
    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return ""
    }
    defer conn.Close()

    reply, err = conn.Do("HGET", redis.Config.Prefix + key + redis.Config.Suffix, hkey)
    if err != nil {
        return ""
    }
    val, err = redisCon.String(reply, nil)
    if err != nil {
        return ""
    }
    return val
}

func (redis *Redis) HSet(key string, hkey string, value string) error {
    conn := redis.Pool.Get()
    if conn == nil {
        log.Printf("[ERROR] cannot connect to redis")
        return errors.New("connection to redis failed")
    }
    defer conn.Close()

    // log.Printf("[DEBUG] HSET : %s %s %s", redis.config.prefix + key + redis.config.suffix, hkey, value)
    _, err := conn.Do("HSET", redis.Config.Prefix + key + redis.Config.Suffix, hkey, value)
    if err != nil {
        log.Printf("[ERROR] redis error : %s", err)
        return err
    }
    return nil
}
