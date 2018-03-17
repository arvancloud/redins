package redis

import (
    "fmt"
    "strings"
    "time"
    "log"

    redisCon "github.com/garyburd/redigo/redis"
    "github.com/go-ini/ini"
    "strconv"
    "errors"
)

type Redis struct {
    config       *RedisConfig
    Pool         *redisCon.Pool
    RedisAddress string
}

type RedisConfig struct {
    ip             string
    port           int
    password       string
    prefix         string
    suffix         string
    connectTimeout int
    readTimeout    int
}

func LoadConfig(cfg *ini.File, section string) *RedisConfig {
    redisConfig := cfg.Section(section)
    return &RedisConfig{
        ip:             redisConfig.Key("ip").MustString("127.0.0.1"),
        port:           redisConfig.Key("port").MustInt(6379),
        password:       redisConfig.Key("password").MustString(""),
        prefix:         redisConfig.Key("prefix").MustString(""),
        suffix:         redisConfig.Key("suffix").MustString(""),
        connectTimeout: redisConfig.Key("connect_timeout").MustInt(0),
        readTimeout:    redisConfig.Key("read_timeout").MustInt(0),
    }

}

func NewRedis(config *RedisConfig) *Redis {
    r := &Redis {
        config: config,
        RedisAddress:   config.ip + ":" + strconv.Itoa(config.port),
    }

    r.Connect()
    return r
}

func (redis *Redis) Connect() {
    redis.Pool = &redisCon.Pool{
        Dial: func() (redisCon.Conn, error) {
            opts := []redisCon.DialOption{}
            if redis.config.password != "" {
                opts = append(opts, redisCon.DialPassword(redis.config.password))
            }
            if redis.config.connectTimeout != 0 {
                opts = append(opts, redisCon.DialConnectTimeout(time.Duration(redis.config.connectTimeout)*time.Millisecond))
            }
            if redis.config.readTimeout != 0 {
                opts = append(opts, redisCon.DialReadTimeout(time.Duration(redis.config.readTimeout)*time.Millisecond))
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

    reply, err = conn.Do("GET", redis.config.prefix + key + redis.config.suffix)
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

    _, err := conn.Do("SET", redis.config.prefix + key + redis.config.suffix, value)
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

    conn.Do("EVAL", "return redis.call('del', unpack(redis.call('keys', ARGV[1])))", 0, redis.config.prefix + pattern + redis.config.suffix)
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

    reply, err = conn.Do("KEYS", redis.config.prefix + "*" + redis.config.suffix)
    if err != nil {
        return nil
    }
    keys, err = redisCon.Strings(reply, nil)
    for i, _ := range keys {
        keys[i] = strings.TrimPrefix(keys[i], redis.config.prefix)
        keys[i] = strings.TrimSuffix(keys[i], redis.config.suffix)
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
        fmt.Println("[ERROR] cannot connect to redis")
        return nil
    }
    defer conn.Close()

    reply, err = conn.Do("HKEYS", redis.config.prefix + key + redis.config.suffix)
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

    reply, err = conn.Do("HGET", redis.config.prefix + key + redis.config.suffix, hkey)
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

    log.Printf("[INFO] HSET : %s %s %s", redis.config.prefix + key + redis.config.suffix, hkey, value)
    _, err := conn.Do("HSET", redis.config.prefix + key + redis.config.suffix, hkey, value)
    if err != nil {
        log.Printf("[ERROR] redis error : %s", err)
        return err
    }
    return nil
}
