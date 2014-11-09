package websocket

import (
	"bytes"
	"io"
	"net/url"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"honnef.co/go/js/dom"
)

func beginHandlerOpen(ch chan error) func(ev js.Object) {
	return func(ev js.Object) {
		close(ch)
	}
}

func beginHandlerClose(ch chan error) func(ev js.Object) {
	return func(ev js.Object) {
		ch <- &js.Error{Object: ev}
		close(ch)
	}
}

// Dial opens a new WebSocket connection. It will block until the connection is
// established or fails to connect.
func Dial(url string) (*Conn, error) {
	ws, err := New(url)
	if err != nil {
		return nil, err
	}

	openCh := make(chan error, 1)

	// We have to use variables for the functions so that we can remove the
	// event handlers afterwards.
	openHandler := beginHandlerOpen(openCh)
	closeHandler := beginHandlerClose(openCh)

	ws.AddEventListener("open", false, openHandler)
	ws.AddEventListener("close", false, closeHandler)

	err, ok := <-openCh

	ws.RemoveEventListener("open", false, openHandler)
	ws.RemoveEventListener("close", false, closeHandler)

	if ok && err != nil {
		return nil, err
	}

	conn := &Conn{
		WebSocket: ws,
		ch:        make(chan *dom.MessageEvent, 1),
	}
	conn.Initialize()

	return conn, nil
}

// Conn is a high-level wrapper around WebSocket. It is intended to
// provide a net.TCPConn-like interface.
type Conn struct {
	*WebSocket

	ch      chan *dom.MessageEvent
	readBuf *bytes.Reader
}

func (c *Conn) onMessage(event js.Object) {
	go func() {
		c.ch <- dom.WrapEvent(event).(*dom.MessageEvent)
	}()
}

func (c *Conn) onClose(event js.Object) {
	go func() {
		// We queue nil to the end so that any messages received prior to
		// closing get handled first.
		c.ch <- nil
	}()
}

// Initialize adds all of the event handlers necessary for a Conn to function.
// It should never be called more than once and is already called if you used
// Dial to create the Conn.
func (c *Conn) Initialize() {
	// We need this so that received binary data is in ArrayBufferView format so
	// that it can easily be read.
	c.BinaryType = "arraybuffer"

	c.AddEventListener("message", false, c.onMessage)
	c.AddEventListener("close", false, c.onClose)
}

// receiveFrame receives one full frame from the WebSocket. It blocks until the
// frame is received.
func (c *Conn) receiveFrame() (*dom.MessageEvent, error) {
	item, ok := <-c.ch
	if !ok { // The channel has been closed
		return nil, io.EOF
	} else if item == nil {
		// See onClose for the explanation about sending a nil item.
		close(c.ch)
		return nil, io.EOF
	}
	return item, nil
}

func getFrameData(obj js.Object) []byte {
	// TODO(nightexcessive): Is there a better way to do this?

	frameStr := obj.Str()
	if frameStr == "[object ArrayBuffer]" {
		int8Array := js.Global.Get("Uint8Array").New(obj)
		return int8Array.Interface().([]byte)
	}

	return []byte(frameStr)
}

func (c *Conn) Read(b []byte) (n int, err error) {
	// TODO(nightexcessive): Implement the deadline functions.

	if c.readBuf != nil {
		n, err = c.readBuf.Read(b)
		if err == io.EOF {
			c.readBuf = nil
			err = nil
		}
		// If we read nothing from the buffer, continue to trying to receive.
		// This saves us when the last Read call emptied the buffer and this
		// call triggers the EOF. There's probably a better way of doing this,
		// but I'm really tired.
		if n > 0 {
			return
		}
	}

	frame, err := c.receiveFrame()
	if err != nil {
		return 0, err
	}

	receivedBytes := getFrameData(frame.Data)

	n = copy(b, receivedBytes)
	// Fast path: The entire frame's contents have been copied into b.
	if n >= len(receivedBytes) {
		return
	}

	c.readBuf = bytes.NewReader(receivedBytes[n:])
	return
}

// Write writes the contents of b to the WebSocket using a binary opcode.
func (c *Conn) Write(b []byte) (n int, err error) {
	// []byte is converted to an (U)Int8Array by GopherJS, which fullfils the
	// ArrayBufferView definition.
	err = c.Send(b)
	if err != nil {
		return
	}
	n = len(b)
	return
}

// WriteString writes the contents of s to the WebSocket using a text frame
// opcode.
func (c *Conn) WriteString(s string) (n int, err error) {
	err = c.Send(s)
	if err != nil {
		return
	}
	n = len(s)
	return
}

// BUG(nightexcessive): We can't return net.Addr from Conn.LocalAddr and
// Conn.RemoteAddr because net.init() causes a panic due to attempts to make
// syscalls. See: https://github.com/gopherjs/gopherjs/issues/123

// LocalAddr would typically return the local network address, but due to
// limitations in the JavaScript API, it is unable to. Calling this method will
// cause a panic.
func (c *Conn) LocalAddr() *Addr {
	// BUG(nightexcessive): Conn.LocalAddr() panics because the underlying
	// JavaScript API has no way of figuring out the local address.

	// TODO(nightexcessive): Find a more graceful way to handle this
	panic("we are unable to implement websocket.Conn.LocalAddr() due to limitations in the underlying JavaScript API")
}

// RemoteAddr returns the remote network address, based on
// websocket.WebSocket.URL.
func (c *Conn) RemoteAddr() *Addr {
	wsURL, err := url.Parse(c.URL)
	if err != nil {
		// TODO(nightexcessive): Should we be panicking for this?
		panic(err)
	}
	return &Addr{wsURL}
}

// SetDeadline implements the net.Conn.SetDeadline method.
func (c *Conn) SetDeadline(t time.Time) error {
	// TODO(nightexcessive): Implement
	panic("not yet implemeneted")
}

// SetReadDeadline implements the net.Conn.SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	// TODO(nightexcessive): Implement
	panic("not yet implemeneted")
}

// SetWriteDeadline implements the net.Conn.SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	// TODO(nightexcessive): Implement
	panic("not yet implemeneted")
}
