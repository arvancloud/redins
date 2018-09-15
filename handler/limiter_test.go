package handler

import (
    "testing"
    "fmt"
    "time"
)

func TestLimiter(t *testing.T) {
    cfg := RateLimiterConfig {
        Enable: true,
        Rate: 60000,
        Burst: 10,
        WhiteList: []string{"w1", "w2"},
        BlackList: []string{"b1", "b2"},
    }
    rl := NewRateLimiter(&cfg)

    fail := 0
    success := 0
    for i := 0; i< 10; i++ {
        if rl.CanHandle("1") == false {
            fail++
        } else {
            success++
        }
    }
    fmt.Println("fail : ", fail, " success : ", success)
    if fail != 0 {
        t.Fail()
    }
    fail = 0
    success = 0
    for i := 0; i< 20; i++ {
        if rl.CanHandle("2") == false {
            fail++
        } else {
            success++
        }
    }
    fmt.Println("fail : ", fail, " success : ", success)
    if fail != 9 || success != 11 {
        t.Fail()
    }

    if rl.CanHandle("b1") == true {
        t.Fail()
    }
    if rl.CanHandle("b2") == true {
        t.Fail()
    }

    for i := 0; i < 100; i++ {
        if rl.CanHandle("w1") == false {
            t.Fail()
        }
        if rl.CanHandle("w2") == false {
            t.Fail()
        }
    }

    fail = 0
    success = 0
    for i := 0; i < 10; i++ {
        if rl.CanHandle("3") != true {
            t.Fail()
        }
    }
    for i := 0; i < 100; i++ {
        time.Sleep(time.Millisecond)
        if rl.CanHandle("3") != true {
            t.Fail()
        }
    }
    fmt.Println("fail : ", fail, " success : ", success)
}