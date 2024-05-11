package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/encoding"
	ws "github.com/gorilla/websocket"
)

const (
	KindWebsocket transport.Kind = "websocket"
	loggerName                   = "transport/websocket"
)

type Binder func() Message

type ConnectHandler func(SessionID, bool)

type MessageHandler func(SessionID, Message) error

type HandlerData struct {
	Handler MessageHandler
	Binder  Binder
}
type MessageHandlerMap map[MessageCmd]*HandlerData

// Logger with server logger.
func Logger(logger log.Logger) ServerOption {
	return func(s *Server) {
		s.log = log.NewHelper(loggerName, logger)
	}
}

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
	log     *log.Helper
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
		log:     log.NewHelper(loggerName, log.DefaultLogger),
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

func (s *Server) SendMessage(sessionId SessionID, message Message) error {
	c, ok := s.sessionMgr.Get(sessionId)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	buf, err := s.marshalMessage(message)
	if err != nil {
		return err
	}

	c.SendMessage(buf)
	return nil
}

func (s *Server) Broadcast(message Message) error {
	buf, err := s.marshalMessage(message)
	if err != nil {
		return err
	}

	s.sessionMgr.Range(func(session *Session) {
		session.SendMessage(buf)
	})
	return nil
}

func (s *Server) marshalMessage(message Message) ([]byte, error) {
	var (
		codecJsonMsg []byte
		//		msgWithLength []byte
		err error
	)

	codecJsonMsg, err = CodecMarshal(s.codec, message)
	if err != nil {
		return nil, err
	}

	//msgWithLength, err = LengthMarshal(codecJsonMsg, s.msgType)
	//if err != nil {
	//	return nil, err
	//}

	//LogInfo("msgWithLength:", string(msgWithLength))

	return codecJsonMsg, nil
}

func (s *Server) unmarshalMessage(msgWithLength []byte) (*HandlerData, Message, error) {
	var (
		//	msgWithoutLength []byte
		//	length           uint32
		handler *HandlerData
		message Message
		baseMsg BaseMsg
		ok      bool
		err     error
	)

	//msgWithoutLength, length, err = LengthUnmarshal(msgWithLength, s.msgType)
	//if err != nil {
	//	return nil, nil, fmt.Errorf("lengthUnmarshal message exception:%v", err)
	//}
	//
	//if int(length) != len(msgWithoutLength) {
	//	return nil, nil, fmt.Errorf("incomplete message")
	//}

	err = CodecUnmarshal(s.codec, msgWithLength, &baseMsg)
	if err != nil {
		return nil, nil, fmt.Errorf("parse the Json command failed:%v", err)
	}

	handler, ok = s.messageHandlers[baseMsg.Command]
	if !ok {
		return nil, nil, errors.New("message handler not found")
	}

	if handler.Binder != nil {
		message = handler.Binder()
		err = CodecUnmarshal(s.codec, msgWithLength, &message)
		if err != nil {
			return nil, nil, fmt.Errorf("parse the Json failed:%v", err)
		}
	} else {
		return nil, nil, errors.New("message Binder not found")
	}

	return handler, message, nil
}

func (s *Server) messageHandler(sessionId SessionID, buf []byte) error {
	var err error
	var handler *HandlerData
	var message Message

	if handler, message, err = s.unmarshalMessage(buf); err != nil {
		return err
	}

	if err = handler.Handler(sessionId, message); err != nil {
		return err
	}

	return nil
}

func (s *Server) wsHandler(res http.ResponseWriter, req *http.Request) {
	conn, err := s.upgrader.Upgrade(res, req, nil)
	if err != nil {
		return
	}

	session := NewSession(conn, s)
	session.server.register <- session

	session.Listen()
	return
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
	s.log.Infof("[websocket] server listening on: %s", s.lis.Addr().String())
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
	s.log.Info("[websocket] server stopping")
	return s.Shutdown(context.Background())
}
