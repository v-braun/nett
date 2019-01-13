package conn

import (
	"io"
	"net"
	"sync"
)

var onDataNop = func(conn Connection, data []byte) {}
var onErrNop = func(conn Connection, err error) {}
var onClosedNop = func(conn Connection) {}

type Connection interface {
	Raw() net.Conn
	OnData(handler func(conn Connection, data []byte))
	OnErr(handler func(conn Connection, err error))
	OnClosed(handler func(conn Connection))
	Send(data []byte) error
	SendAsync(data []byte)
	Close()
}

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

type conn struct {
	inner    net.Conn
	onData   (func(conn Connection, data []byte))
	onErr    (func(conn Connection, err error))
	onClosed (func(conn Connection))
	reader   (func(rawConn net.Conn) ([]byte, error))
	mutex    sync.Mutex
	wg       sync.WaitGroup
}

func Wrap(inner net.Conn, reader func(rawConn net.Conn) ([]byte, error)) Connection {
	result := &conn{
		inner:    inner,
		onData:   onDataNop,
		onErr:    onErrNop,
		onClosed: onClosedNop,
		reader:   reader,
		mutex:    sync.Mutex{},
		wg:       sync.WaitGroup{},
	}

	result.wg.Add(1)
	go result.runRead()

	return result
}

func (c *conn) Raw() net.Conn {
	return c.inner
}
func (c *conn) OnData(handler func(conn Connection, data []byte)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if handler == nil {
		handler = onDataNop
	}

	c.onData = handler
}
func (c *conn) OnErr(handler func(conn Connection, err error)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if handler == nil {
		handler = onErrNop
	}

	c.onErr = handler
}
func (c *conn) OnClosed(handler func(conn Connection)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if handler == nil {
		handler = onClosedNop
	}

	c.onClosed = handler
}

func (c *conn) Send(data []byte) error {
	_, err := c.inner.Write(data)
	if err := hideTempError(err); err != nil {
		c.inner.Close()
		return err
	}
	return nil
}

func (c *conn) SendAsync(data []byte) {
	c.wg.Add(1)
	go func(c *conn, data []byte) {
		defer c.wg.Done()

		err := c.Send(data)
		if err = hideTempError(err); err != nil {
			c.notifyErr(err)
			return
		}
	}(c, data)
}

func (c *conn) Close() {
	c.inner.Close()
	c.wg.Wait()
}

func (c *conn) runRead() {
	defer func() {
		c.inner.Close()
		c.wg.Done()
		go c.notifyClose()
	}()

	for {
		data, err := c.reader(c.inner)
		if err = hideTempError(err); err != nil {
			if isClosedConnErrr(err) {
				return
			}

			c.notifyErr(err)
			return
		}

		if len(data) > 0 {
			c.notifyData(data)
		}
	}
}

func isClosedConnErrr(err error) bool {
	if err == io.EOF {
		return true
	}
	closedConnErr := "use of closed network connection"
	if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == closedConnErr {
		return true
	}

	return false
}

func (c *conn) notifyErr(err error) {
	c.mutex.Lock()
	handler := c.onErr
	c.mutex.Unlock()

	handler(c, err)
}

func (c *conn) notifyData(data []byte) {
	c.mutex.Lock()
	handler := c.onData
	c.mutex.Unlock()

	handler(c, data)
}
func (c *conn) notifyClose() {
	c.mutex.Lock()
	handler := c.onClosed
	c.mutex.Unlock()

	handler(c)
}

func hideTempError(err error) error {
	if err == nil {
		return nil
	}

	if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
		return nil
	}

	return err
}
