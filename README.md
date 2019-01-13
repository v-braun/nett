# nett   
> Go's net.Conn wrapper with simple API

By [v-braun - viktor-braun.de](https://viktor-braun.de).

[![](https://img.shields.io/github/license/v-braun/nett.svg?style=flat-square)](https://github.com/v-braun/nett/blob/master/LICENSE)
[![Build Status](https://img.shields.io/travis/v-braun/nett.svg?style=flat-square)](https://travis-ci.org/v-braun/nett)
[![codecov](https://codecov.io/gh/v-braun/nett/branch/master/graph/badge.svg)](https://codecov.io/gh/v-braun/nett)
![PR welcome](https://img.shields.io/badge/PR-welcome-green.svg?style=flat-square)

<p align="center">
<img width="70%" src="https://via.placeholder.com/800x480.png?text=this%20is%20a%20placeholder%20for%20the%20project%20banner" />
</p>


## Description  

**nett** *(German word for nice)* is a wrapper for the net.Conn interface  
It provides a simple event based interface and supports sync and async send operations

## Installation
```sh
go get github.com/v-braun/nett
```



## Usage

``` golang

var c1, c2 net.Conn = createConnections() // create your connections
client1 := nett.Wrap(c1, nett.ReadLineReader)
client2 := nett.Wrap(c2, nett.ReadLineReader)

client1.OnData(func(conn Connection, data []byte) {
    assert.Equal(t, "ping\n", string(data))
    conn.Send([]byte("pong\n"))
})
client2.OnData(func(conn Connection, data []byte) {
    assert.Equal(t, "pong\n", string(data))
})

client2.SendAsync([]byte("ping\n"))

```

See also the example in [nett_test.go](https://github.com/v-braun/nett/blob/58d050c19512052eef1daa9020285eb48cdafc1d/nett_test.go#L271):

```golang
func ExamplePingPong() {
	wg := &sync.WaitGroup{}

	// create a listener
	srvAddr, _ := net.ResolveTCPAddr("tcp", ":0")
	s, _ := net.Listen("tcp", srvAddr.String())

	// setup async accept for the listener
	srvConnChan := make(chan net.Conn)
	go func() {
		for {
			c, _ := s.Accept()
			if c != nil {
				srvConnChan <- c
			}
		}
	}()

	// dial to the listener above
	clntAddr, _ := net.ResolveTCPAddr("tcp", ":0")
	c1, _ := net.DialTCP("tcp", clntAddr, s.Addr().(*net.TCPAddr))
	c2 := <-srvConnChan

	wg.Add(2) // expect a ping and a pong
	client1 := nett.Wrap(c1, nett.ReadLineReader)
	client2 := nett.Wrap(c2, nett.ReadLineReader)

	client1.OnData(func(c nett.Connection, data []byte) {
		fmt.Print(string(data))
		c.Send([]byte("pong\n"))
		wg.Done()
	})

	client2.OnData(func(c nett.Connection, data []byte) {
		fmt.Print(string(data))
		wg.Done()
	})

	client2.Send([]byte("ping\n"))

	wg.Wait() // wait until ping and pong
	client2.Close()
	client1.Close()

	// Output:
	//ping
	//pong
}
```


### Decode Messages

You have to provide a *reader* callback to the *Wrap()* func.  
This callback will be called in a goroutine and waits until a *[]byte* is returned.  
The result of this callback will be assumed as a complete message and the *OnData* handler will be called.  
A basic implementation of this handler is the [**ReadLineReader**](https://github.com/v-braun/nett/blob/58d050c19512052eef1daa9020285eb48cdafc1d/nett.go#L40):   

``` golang

// ReadLineReader is an reader implementation (see Wrap)
// that reads data line by line from the underlining connection
var ReadLineReader = func(rawConn net.Conn) ([]byte, error) {
	newLine := byte('\n')
	buffer := []byte{}
	readBuffer := make([]byte, 1)
	for {
		_, err := rawConn.Read(readBuffer)
		if err != nil {
			return nil, err
		}
		buffer = append(buffer, readBuffer[0])
		if readBuffer[0] == newLine {
			return buffer, nil
		}
	}
}

```




## Authors

![image](https://avatars3.githubusercontent.com/u/4738210?v=3&amp;s=50)  
[v-braun](https://github.com/v-braun/)



## Contributing

Make sure to read these guides before getting started:
- [Contribution Guidelines](https://github.com/v-braun/nett/blob/master/CONTRIBUTING.md)
- [Code of Conduct](https://github.com/v-braun/nett/blob/master/CODE_OF_CONDUCT.md)

## License
**nett** is available under the MIT License. See [LICENSE](https://github.com/v-braun/nett/blob/master/LICENSE) for details.
