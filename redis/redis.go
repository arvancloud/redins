package redis

import (
    "time"
    "strconv"
    "errors"
    "strings"

    redisCon "github.com/gomodule/redigo/redis"
    "arvancloud/redins/eventlog"
)

type Redis struct {
    Config       *RedisConfig
    Pool         *redisCon.Pool
    RedisAddress string
}

type RedisConfig struct {
    Ip             string `json:"ip,omitempty"`
    Port           int    `json:"port,omitempty"`
    Password       string `json:"password,omitempty"`
    Prefix         string `json:"prefix,omitempty"`
    Suffix         string `json:"suffix,omitempty"`
    ConnectTimeout int    `json:"connect_timeout,omitempty"`
    ReadTimeout    int    `json:"read_timeout,omitempty"`
}

func NewRedis(config *RedisConfig) *Redis {
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
        eventlog.Logger.Error("cannot connect to redis")
        return ""
    }
    defer conn.Close()

    reply, err = conn.Do("GET", redis.Config.Prefix + key + redis.Config.Suffix)
    if err != nil {
        eventlog.Logger.Errorf("redis error : GET %s : %s", key, err)
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
        eventlog.Logger.Error("cannot connect to redis")
        return errors.New("connection to redis failed")
    }
    defer conn.Close()

    _, err := conn.Do("SET", redis.Config.Prefix + key + redis.Config.Suffix, value)
    if err != nil {
        eventlog.Logger.Errorf("redis error : SET %s : %s", key, err)
        return err
    }
    return nil
}

func (redis *Redis) Del(pattern string) {
    conn := redis.Pool.Get()
    if conn == nil {
        eventlog.Logger.Error("cannot connect to redis")
        return
    }
    defer conn.Close()

    keys := redis.GetKeys(pattern)
    if keys == nil || len(keys) == 0 {
        eventlog.Logger.Debug("nothing to delete")
        return
    }
    var arg []interface{}
    for i := range keys {
        arg = append(arg, redis.Config.Prefix + keys[i] + redis.Config.Suffix)
    }
    _, err := conn.Do("DEL", arg...)
    if err != nil {
        eventlog.Logger.Errorf("error in redis : DEL : %s : %s", pattern, err)
    }
}

func (redis *Redis) GetKeys(pattern string) []string {
    var (
        reply interface{}
        err   error
        keys []string
    )

    conn := redis.Pool.Get()
    if conn == nil {
        eventlog.Logger.Error("cannot connect to redis")
        return nil
    }
    defer conn.Close()

    keySet := make(map[string]interface{})

    cursor := "0"
    for {
        reply, err = conn.Do("SCAN", cursor, "MATCH", redis.Config.Prefix + pattern + redis.Config.Suffix, "COUNT", 100)
        if err != nil {
            eventlog.Logger.Errorf("redis command failed : SCAN : %s", err)
            return nil
        }
        var values []interface{}
        values, err = redisCon.Values(reply, nil)
        if err != nil {
            eventlog.Logger.Error("cannot get values from reply : ", reply, " : ", err)
            return nil
        }
        cursor, err = redisCon.String(values[0], nil)
        if err != nil {
            eventlog.Logger.Error("cannot convert ", values[0], " to cursor")
            return nil
        }
        keys, err = redisCon.Strings(values[1], nil)
        if err != nil {
            eventlog.Logger.Error("cannot get keys from ", values[1])
            return nil
        }
        for _, key := range keys {
            keySet[key] = nil
        }
        if cursor == "0" {
            break
        }
    }
    keys = []string{}
    for key := range keySet {
        key = strings.TrimPrefix(key, redis.Config.Prefix)
        key = strings.TrimSuffix(key, redis.Config.Suffix)
        keys = append(keys, key)
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
        eventlog.Logger.Error("cannot connect to redis")
        return nil
    }
    defer conn.Close()

    reply, err = conn.Do("HKEYS", redis.Config.Prefix + key + redis.Config.Suffix)
    if err != nil {
        eventlog.Logger.Errorf("error in redis command : HKEYS %s : %s", key, err)
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
        eventlog.Logger.Error("cannot connect to redis")
        return ""
    }
    defer conn.Close()

    reply, err = conn.Do("HGET", redis.Config.Prefix + key + redis.Config.Suffix, hkey)
    if err != nil {
        eventlog.Logger.Errorf("redis error : HGET %s : %s", key, err)
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
        eventlog.Logger.Error("cannot connect to redis")
        return errors.New("connection to redis failed")
    }
    defer conn.Close()

    // log.Printf("[DEBUG] HSET : %s %s %s", redis.config.prefix + key + redis.config.suffix, hkey, value)
    _, err := conn.Do("HSET", redis.Config.Prefix + key + redis.Config.Suffix, hkey, value)
    if err != nil {
        eventlog.Logger.Errorf("redis error : HSET %s : %s", key, err)
        return err
    }
    return nil
}
