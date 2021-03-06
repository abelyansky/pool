# Pool

A  thread safe connection pool for any kinds of connections. It enforces blocking on the get requests when the pool is exhausted.
Additionally it provides an http client adapter which behaves like a regular http.Client but uses a pool of http.Client instances
underneath. Additional adapters can be built for other types of connections using this pattern.

## Install and Usage

Install gvt if you don't have it
```bash
go get github.com/FiloSottile/gvt
```

Install the package with:
```bash
go get github.com/abelyansky/pool
```

Bring in dependencies
```bash
gvt restore
```

Import it with:

```go
import "github.com/abelyansky/pool"
```

and use `pool` as the package name inside the code.

## Example of using a generic pool

```go
// create a factory() to be used with channel based pool
factory    := func() (pool.GenericConnection, error) 
 {  
   return net.Dial("tcp", "127.0.0.1:4000").(GenericConnection) 
 }

// create a new channel based pool with a maximum capacity of 30. 
// The factory will create 30 initial connections and put them
// into the pool.
p, err := pool.NewChannelPool(30, factory)

// now you can get a connection holder from the pool referencing the connection.
// if there is no connection available the call will block
connWrapper, err := p.Get()

// or specify a timeout to avoid blocking indefinitely
// in case of a timeout err will be set to pool.ErrTimedOut
connWrapper, err := p.GetWithTimeout(duration)

// do something with conn and put it back to the pool
// connWrapper.Conn.(*net.TCPConn).Write(...)
p.Put(connWrapper)

// close pool any time you want, this closes all the connections inside a pool
p.Close()

// currently available connections in the pool
current := p.Len()
```

## Example of using an http pool adapter

```go
// create an http client factory
httpClientFactory = func() (adapters.HttpClient, error) {
		return &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 1,
				// to make a point that this is what we want
				DisableKeepAlives:     false,
				ExpectContinueTimeout: 15 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
			},
			Timeout: 15 * time.Second,
		}, nil
	}
// instantiate a pool
pooledHttpClient := adapters.NewPooledHttpClient(10, httpClientFactory)
// use it as you a regulal http.Client
// then cleanup when you are done
pooledHttpClient.Cleanup()
```

## License

The MIT License (MIT) - see LICENSE for more details
