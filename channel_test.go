package pool

import (
	"log"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
	"io/ioutil"
	"bytes"
	"fmt"
	"strconv"
)

var (
	InitialCap = 5
	MaximumCap = 30
	address    = "http://127.0.0.1:7777"

	factory    = func() (GenericConn, error) { return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 1,
			// to make a point that this is what we want
			DisableKeepAlives: false,
		},
	}, nil }
)

func init() {
	// used for factory function
	go simpleTCPServer()
	time.Sleep(time.Millisecond * 300) // wait until tcp server has been settled

	rand.Seed(time.Now().UTC().UnixNano())
}

func TestNew(t *testing.T) {
	_, err := newChannelPool()
	if err != nil {
		t.Errorf("New error: %s", err)
	}
}
func TestPool_Get_Impl(t *testing.T) {
	p, _ := newChannelPool()
	defer p.Close()

	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	_, ok := conn.(*http.Client)
	if !ok {
		t.Errorf("Conn is not of type poolConn")
	}
}

func TestPool_Get(t *testing.T) {
	p, _ := newChannelPool()
	defer p.Close()

	_, err := p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}

	// after one get, current capacity should be lowered by one.
	if p.Len() != (InitialCap - 1) {
		t.Errorf("Get error. Expecting %d, got %d",
			(InitialCap - 1), p.Len())
	}

	// get them all
	var wg sync.WaitGroup
	for i := 0; i < (InitialCap - 1); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Get()
			if err != nil {
				t.Errorf("Get error: %s", err)
			}
		}()
	}
	wg.Wait()

	if p.Len() != 0 {
		t.Errorf("Get error. Expecting %d, got %d",
			(InitialCap - 1), p.Len())
	}

	_, err = p.Get()
	if err != nil {
		t.Errorf("Get error: %s", err)
	}
}

func TestPool_Put(t *testing.T) {
	p, err := NewChannelPool(0, 30, factory)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	// get/create from the pool
	conns := make([]GenericConn, MaximumCap)
	for i := 0; i < MaximumCap; i++ {
		conn, _ := p.Get()
		conns[i] = conn
	}

	// now put them all back
	for _, conn := range conns {
		p.Put(conn)
	}

	if p.Len() != MaximumCap {
		t.Errorf("Put error len. Expecting %d, got %d",
			1, p.Len())
	}

	conn, _ := p.Get()
	p.Close() // close pool

	p.Put(conn)
	if p.Len() != 0 {
		t.Errorf("Put error. Closed pool shouldn't allow to put connections.")
	}
}

func TestPool_PutUnusableConn(t *testing.T) {
	p, _ := newChannelPool()
	defer p.Close()

	// ensure pool is not empty
	conn, _ := p.Get()
	p.Put(conn)

	poolSize := p.Len()
	conn, _ = p.Get()
	p.Put(conn)
	if p.Len() != poolSize {
		t.Errorf("Pool size is expected to be equal to initial size")
	}

	conn, _ = p.Get()
	if _, ok := conn.(GenericConn); !ok {
		t.Errorf("impossible")
	}

	if p.Len() != poolSize-1 {
		t.Errorf("Pool size is expected to be initial_size - 1", p.Len(), poolSize-1)
	}
}

func TestPool_UsedCapacity(t *testing.T) {
	p, _ := newChannelPool()
	defer p.Close()

	if p.Len() != InitialCap {
		t.Errorf("InitialCap error. Expecting %d, got %d",
			InitialCap, p.Len())
	}
}

func TestPool_Close(t *testing.T) {
	p, _ := newChannelPool()

	// now close it and test all cases we are expecting.
	p.Close()

	c := p.(*channelPool)

	if c.conns != nil {
		t.Errorf("Close error, conns channel should be nil")
	}

	if c.factory != nil {
		t.Errorf("Close error, factory should be nil")
	}

	_, err := p.Get()
	if err == nil {
		t.Errorf("Close error, get conn should return an error")
	}

	if p.Len() != 0 {
		t.Errorf("Close error used capacity. Expecting 0, got %d", p.Len())
	}
}

func TestPoolConcurrent(t *testing.T) {
	p, _ := newChannelPool()
	pipe := make(chan GenericConn, 0)

	go func() {
		p.Close()
	}()

	for i := 0; i < MaximumCap; i++ {
		go func() {
			conn, _ := p.Get()

			pipe <- conn
		}()

		go func() {
			conn := <-pipe
			if conn == nil {
				return
			}
			p.Put(conn)
		}()
	}
}

func TestPoolWriteRead(t *testing.T) {
	p, _ := NewChannelPool(0, 30, factory)

	conn, _ := p.Get()

	msg := "hello"
	resp, err := conn.(*http.Client).Post("http://localhost:7777/echo", "text/plain", bytes.NewReader([]byte(msg)))
	if err != nil {
		t.Error(err)
	}
	respMsg, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	} else {
		defer resp.Body.Close()
	}

	if msg != string(respMsg) {
		t.Errorf("Expected response %s but got %s ", msg, resp)
	}
}

func TestPoolConcurrent2(t *testing.T) {
	p, _ := NewChannelPool(0, 30, factory)

	var wg sync.WaitGroup

	go func() {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				conn, _ := p.Get()
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
				p.Put(conn)
				wg.Done()
			}(i)
		}
	}()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			conn, _ := p.Get()
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
			p.Put(conn)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func newChannelPool() (Pool, error) {
	return NewChannelPool(InitialCap, MaximumCap, factory)
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeHTTP called")

	queryArgs := r.URL.Query()

	data, err := ioutil.ReadAll(r.Body)
	if err == nil {
		defer r.Body.Close()
	}

	if sleepDur, ok := queryArgs["sleep"]; ok {
		sleepDurSec, err := strconv.Atoi(sleepDur[0])
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Duration(sleepDurSec) * time.Second)
	}
	w.Write(data)
}


func simpleTCPServer() {
	http.HandleFunc("/echo", ServeHTTP)
	err := http.ListenAndServe(":7777", nil)

	if err != nil {
		log.Fatal(err)
	}
}
