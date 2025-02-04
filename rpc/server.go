package rpc

import (
	"io"
	"log"

	"github.com/progrium/qtalk-go/codec"
	"github.com/progrium/qtalk-go/mux"
	"github.com/progrium/qtalk-go/transport"
)

// Server wraps a Handler and codec to respond to RPC calls.
type Server struct {
	Handler Handler
	Codec   codec.Codec
}

// Serve will Accept sessions until the Listener is closed and Respond to accepted sessions in their own goroutine.
func (s *Server) Serve(l transport.Listener) error {
	for {
		sess, err := l.Accept()
		if err != nil {
			return err
		}
		go s.Respond(sess)
	}
}

// Respond will Accept channels until the Session is closed and respond with the server handler in its own goroutine.
// If Handler was not set, an empty RespondMux is used. If the handler does not initiate a response, a nil value is
// returned. If the handler does not call Continue, the channel will be closed.
func (s *Server) Respond(sess *mux.Session) {
	defer sess.Close()
	for {
		ch, err := sess.Accept()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		go s.respond(sess, ch)
	}
}

func (s *Server) respond(sess *mux.Session, ch *mux.Channel) {
	framer := &codec.FrameCodec{Codec: s.Codec}
	dec := framer.Decoder(ch)

	var call Call
	err := dec.Decode(&call)
	if err != nil {
		log.Println("rpc.Respond:", err)
		return
	}

	call.Decoder = dec
	call.Caller = &Client{
		Session: sess,
		codec:   s.Codec,
	}

	header := &ResponseHeader{}
	resp := &responder{
		ch:     ch,
		c:      framer,
		header: header,
	}

	if s.Handler == nil {
		s.Handler = NewRespondMux()
	}

	s.Handler.RespondRPC(resp, &call)
	if !resp.responded {
		resp.Return(nil)
	}
	if !resp.header.Continue {
		ch.Close()
	}
}
