package nett_test

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/v-braun/nett"
)

var expectedReadErr = errors.New("READ")
var expectedWriteErr = errors.New("WRITE")

var mockMsg = []byte("hello world")
var _ net.Conn = &mockConn{}

type mockConn struct {
	readErr  error
	writeErr error
}

var _ net.Error = &mockTempError{}

type mockTempError struct {
	error
}

func newMockTempErr() error {
	return &mockTempError{error: errors.New("MOCK")}
}

func (e *mockTempError) Temporary() bool {
	return true
}
func (e *mockTempError) Timeout() bool {
	return false
}

func (c *mockConn) Close() error { return nil }

func (c *mockConn) Read(b []byte) (n int, err error) { return 0, c.readErr }

func (c *mockConn) Write(b []byte) (n int, err error) { return 0, c.writeErr }

func (c *mockConn) LocalAddr() net.Addr { return nil }

func (c *mockConn) RemoteAddr() net.Addr { return nil }

func (c *mockConn) SetDeadline(t time.Time) error { return nil }

func (c *mockConn) SetReadDeadline(t time.Time) error { return nil }

func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestWrap(t *testing.T) {
	client1, client2 := createSUTs(t)

	assert.NotNil(t, client1.Raw())
	assert.NotNil(t, client2.Raw())

	wg := &sync.WaitGroup{}

	wg.Add(1)
	client1.OnClosed(func(conn nett.Connection) {
		wg.Done()
	})

	wg.Add(1)
	client2.OnClosed(func(conn nett.Connection) {
		wg.Done()
	})

	client1.Close()
	client2.Close()
	wg.Wait()
}

func TestSend(t *testing.T) {
	client1, client2 := createSUTs(t)

	sendData := mockMsg
	receiveData := make([]byte, len(sendData))

	wg := &sync.WaitGroup{}

	wg.Add(1)
	client2.OnData(func(conn nett.Connection, data []byte) {
		receiveData = data
		wg.Done()
	})

	client1.Send(sendData)
	wg.Wait()

	sended := string(sendData)
	received := string(receiveData)
	assert.Equal(t, sended, received)

	client1.Close()
	client2.Close()
}

func TestSendAsync(t *testing.T) {
	client1, client2 := createSUTs(t)

	sendData := mockMsg
	receiveData := make([]byte, len(sendData))

	wg := &sync.WaitGroup{}

	wg.Add(1)
	client2.OnData(func(conn nett.Connection, data []byte) {
		receiveData = data
		wg.Done()
	})

	client1.SendAsync(sendData)
	wg.Wait()

	sended := string(sendData)
	received := string(receiveData)
	assert.Equal(t, sended, received)

	client1.Close()
	client2.Close()
}

func TestErr(t *testing.T) {
	client1, client2 := createSUTs(t)

	wg := &sync.WaitGroup{}

	var expectedErr1 error = nil
	wg.Add(1)
	client1.OnErr(func(conn nett.Connection, err error) {
		expectedErr1 = err
		wg.Done()
	})
	wg.Add(1)
	client1.OnClosed(func(conn nett.Connection) {
		wg.Done()
	})

	wg.Add(1)
	client2.OnClosed(func(conn nett.Connection) {
		wg.Done()
	})

	client1.Close()
	client1.SendAsync(mockMsg)
	wg.Wait()

	assert.Error(t, expectedErr1)
	assert.Error(t, expectedErr1)

	client2.Close()
}

func TestNillableHandlers(t *testing.T) {
	client1, client2 := createSUTs(t)
	client1.OnClosed(nil)
	client1.OnErr(nil)
	client1.OnData(nil)

	client1.Send(mockMsg)

	client1.Close()
	client2.Close()
}

func TestReadAll(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	c1, c2 := createClients(t)
	client1 := nett.Wrap(c1, nett.ReadLineReader)
	client2 := nett.Wrap(c2, nett.ReadLineReader)

	client1.OnData(func(conn nett.Connection, data []byte) {
		assert.Equal(t, "ping\n", string(data))
		conn.Send([]byte("pong\n"))

		wg.Done()
	})
	client2.OnData(func(conn nett.Connection, data []byte) {
		assert.Equal(t, "pong\n", string(data))
		wg.Done()
	})

	client2.SendAsync([]byte("ping\n"))
	wg.Wait()
	client1.Close()
	client2.Close()
}
func TestTempError(t *testing.T) {
	wgReadTmpErr := &sync.WaitGroup{}
	wgReadRealErr := &sync.WaitGroup{}
	wgErrorSended := &sync.WaitGroup{}
	wgErrorHandler := &sync.WaitGroup{}
	wgClosedHandler := &sync.WaitGroup{}

	wgReadTmpErr.Add(1)
	wgReadRealErr.Add(1)
	wgErrorSended.Add(1)
	wgErrorHandler.Add(1)
	wgClosedHandler.Add(1)

	readTmpReturned := false
	reader := func(rawConn net.Conn) ([]byte, error) {
		if readTmpReturned == false {
			wgReadTmpErr.Done()
			readTmpReturned = true
			return nil, newMockTempErr()
		}

		wgErrorSended.Wait()
		wgReadRealErr.Done()
		return nil, expectedReadErr
	}

	conn := &mockConn{writeErr: expectedWriteErr, readErr: nil}
	c := nett.Wrap(conn, reader)

	c.OnErr(func(conn nett.Connection, err error) {
		wgErrorHandler.Done()
		if err != expectedReadErr && err != expectedWriteErr {
			assert.FailNow(t, "unexpected err")
		}
	})

	c.OnClosed(func(conn nett.Connection) {
		wgClosedHandler.Done()
	})

	c.Send(mockMsg)
	wgErrorSended.Done()

	wgReadTmpErr.Wait()
	wgReadRealErr.Wait()
	wgErrorSended.Wait()
	wgErrorHandler.Wait()
	wgClosedHandler.Wait()

}

func ExampleWrap() {
	wg := &sync.WaitGroup{}

	// create a listener
	srvAddr, _ := net.ResolveTCPAddr("tcp", "localhost:0")
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
	clntAddr, _ := net.ResolveTCPAddr("tcp", "localhost:0")
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

func createReader(expectedMsg []byte) func(rawConn net.Conn) ([]byte, error) {
	result := func(rawConn net.Conn) ([]byte, error) {
		data := make([]byte, len(mockMsg))
		_, err := rawConn.Read(data)

		return data, err
	}

	return result
}

func accept(t *testing.T, listener net.Listener) chan net.Conn {
	c := make(chan net.Conn)
	go func(c chan net.Conn) {
		conn, err := listener.Accept()

		if err != nil {
			t.Error(err)
		}

		c <- conn

	}(c)

	return c
}

func createSUTs(t *testing.T) (nett.Connection, nett.Connection) {
	// c1, c2 := net.Pipe() could be used to check pipes / without a client / server
	read := createReader(mockMsg)

	c1, c2 := createClients(t)

	client1 := nett.Wrap(c1, read)
	client2 := nett.Wrap(c2, read)

	return client1, client2
}

func createClients(t *testing.T) (net.Conn, net.Conn) {
	server, serverAddr := createListener(t)
	connChan := accept(t, server)
	client2 := createClient(t, serverAddr)
	client1 := <-connChan
	assert.NotNil(t, client2)
	assert.NotNil(t, client1)
	if client1 == nil || client2 == nil {
		t.FailNow()
	}

	return client1, client2
}

func createClient(t *testing.T, remote net.Addr) net.Conn {
	localAddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	c, err := net.DialTCP("tcp", localAddr, remote.(*net.TCPAddr))

	if err != nil {
		t.Error(err)
		return nil
	}

	return c
}

func createListener(t *testing.T) (net.Listener, net.Addr) {
	localAddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		t.Error(err)
		return nil, nil
	}

	server, err := net.Listen("tcp", localAddr.String())
	if err != nil {
		t.Error(err)
		return nil, nil
	}

	return server, server.Addr()

}
