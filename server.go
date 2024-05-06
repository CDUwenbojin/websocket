package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/transport"

	ws "github.com/gorilla/websocket"
)

type Binder func() Message

type ConnectHandler func(SessionID, bool)

type MessageHandler func(SessionID, Message) error

type HandlerData struct {
	Handler MessageHandler
	Binder  Binder
}
type MessageHandlerMap map[MessageCmd]*HandlerData

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

type Server struct {
	*http.Server

	lis      net.Listener
	tlsConf  *tls.Config
	upgrader *ws.Upgrader

	network     string
	address     string
	path        string
	strictSlash bool

	timeout time.Duration

	err   error
	codec encoding.Codec

	messageHandlers MessageHandlerMap

	sessionMgr *SessionManager

	register   chan *Session
	unregister chan *Session

	msgType MsgType
}

func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		network:     "tcp",
		address:     ":0",
		timeout:     1 * time.Second,
		strictSlash: true,
		path:        "/",

		messageHandlers: make(MessageHandlerMap),

		sessionMgr: NewSessionManager(),
		upgrader: &ws.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},

		register:   make(chan *Session),
		unregister: make(chan *Session),

		msgType: MsgTypeBinary,
	}

	srv.init(opts...)

	srv.err = srv.listen()

	return srv
}

func (s *Server) Name() string {
	return string(KindWebsocket)
}

func (s *Server) init(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}

	s.Server = &http.Server{
		TLSConfig: s.tlsConf,
	}

	http.HandleFunc(s.path, s.wsHandler)
}

func (s *Server) SessionCount() int {
	return s.sessionMgr.Count()
}

func (s *Server) RegisterMessageHandler(cmd MessageCmd, handler MessageHandler, binder Binder) {
	if _, ok := s.messageHandlers[cmd]; ok {
		return
	}

	s.messageHandlers[cmd] = &HandlerData{
		handler, binder,
	}
}

func RegisterServerMessageHandler[T any](srv *Server, cmd MessageCmd, handler func(SessionID, *T) error) {
	srv.RegisterMessageHandler(cmd,
		func(sessionId SessionID, message Message) error {
			switch t := message.(type) {
			case *T:
				return handler(sessionId, t)
			default:
				LogError("invalid message struct type:", t)
				return errors.New("invalid message struct type")
			}
		},
		func() Message {
			var t T
			return &t
		},
	)
}

func (s *Server) DeregisterMessageHandler(cmd MessageCmd) {
	delete(s.messageHandlers, cmd)
}

func (s *Server) SendMessage(sessionId SessionID, message Message) {
	c, ok := s.sessionMgr.Get(sessionId)
	if !ok {
		LogError("session not found:", sessionId)
		return
	}

	buf, err := s.marshalMessage(message)
	if err != nil {
		LogError("marshal message exception:", err)
		return
	}

	c.SendMessage(buf)
}

func (s *Server) Broadcast(cmd MessageCmd, message Message) {
	buf, err := s.marshalMessage(message)
	if err != nil {
		LogError(" marshal message exception:", err)
		return
	}

	s.sessionMgr.Range(func(session *Session) {
		session.SendMessage(buf)
	})
}

func (s *Server) marshalMessage(message Message) ([]byte, error) {
	var (
		codecJsonMsg,
		msgWithLength []byte
		err error
	)

	codecJsonMsg, err = CodecMarshal(s.codec, message)
	if err != nil {
		return nil, err
	}

	msgWithLength, err = LengthMarshal(codecJsonMsg, s.msgType)
	if err != nil {
		return nil, err
	}

	//LogInfo("msgWithLength:", string(msgWithLength))

	return msgWithLength, nil
}

func (s *Server) unmarshalMessage(msgWithLength []byte) (*HandlerData, Message, error) {
	var (
		msgWithoutLength []byte
		length           uint32
		handler          *HandlerData
		message          Message
		baseMsg          BaseMsg
		ok               bool
		err              error
	)

	msgWithoutLength, length, err = LengthUnmarshal(msgWithLength, s.msgType)
	if err != nil {
		LogError("lengthUnmarshal message exception:", err)
		return nil, nil, fmt.Errorf("lengthUnmarshal message exception:%v", err)
	}

	if int(length) != len(msgWithoutLength) {
		LogError("incomplete message")
		return nil, nil, fmt.Errorf("incomplete message")
	}

	err = CodecUnmarshal(s.codec, msgWithoutLength, &baseMsg)
	if err != nil {
		LogError("parse the Json command failed:", err)
		return nil, nil, fmt.Errorf("parse the Json command failed:%v", err)
	}

	handler, ok = s.messageHandlers[baseMsg.Command]
	if !ok {
		LogError("message handler not found:", baseMsg.Command)
		return nil, nil, errors.New("message handler not found")
	}

	if handler.Binder != nil {
		message = handler.Binder()
		err = CodecUnmarshal(s.codec, msgWithoutLength, &message)
		if err != nil {
			LogError("parse the Json failed:", err)
			return nil, nil, fmt.Errorf("parse the Json failed:%v", err)
		}
	} else {
		LogError("message Binder not found:", baseMsg.Command)
		return nil, nil, errors.New("message Binder not found")
	}

	return handler, message, nil
}

func (s *Server) messageHandler(sessionId SessionID, buf []byte) error {
	var err error
	var handler *HandlerData
	var message Message

	if handler, message, err = s.unmarshalMessage(buf); err != nil {
		LogErrorf("unmarshal message failed: %s", err)
		return err
	}
	//LogDebug(payload)

	if err = handler.Handler(sessionId, message); err != nil {
		LogErrorf("message handler failed: %s", err)
		return err
	}

	return nil
}

func (s *Server) wsHandler(res http.ResponseWriter, req *http.Request) {
	conn, err := s.upgrader.Upgrade(res, req, nil)
	if err != nil {
		LogError("upgrade exception:", err)
		return
	}

	session := NewSession(conn, s)
	session.server.register <- session

	session.Listen()
}

func (s *Server) listen() error {
	if s.lis == nil {
		lis, err := net.Listen(s.network, s.address)
		if err != nil {
			s.err = err
			return err
		}
		s.lis = lis
	}

	return nil
}

func (s *Server) Endpoint() (string, error) {
	addr := s.address

	prefix := "ws://"
	if s.tlsConf == nil {
		if !strings.HasPrefix(addr, "ws://") {
			prefix = "ws://"
		}
	} else {
		if !strings.HasPrefix(addr, "wss://") {
			prefix = "wss://"
		}
	}
	addr = prefix + addr

	var endpoint *url.URL
	endpoint, s.err = url.Parse(addr)
	return endpoint.String(), nil
}

func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.sessionMgr.Add(client)
		case client := <-s.unregister:
			s.sessionMgr.Remove(client)
		}
	}
}

func (s *Server) Start() error {
	if s.err != nil {
		return s.err
	}
	ctx := context.Background()
	s.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	LogInfof("server listening on: %s", s.lis.Addr().String())

	go s.run()

	var err error
	if s.tlsConf != nil {
		err = s.ServeTLS(s.lis, "", "")
	} else {
		err = s.Serve(s.lis)
	}
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop() error {
	LogInfo("server stopping")
	return s.Shutdown(context.Background())
}
