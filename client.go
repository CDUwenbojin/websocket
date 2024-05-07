package websocket

import (
	"errors"
	"fmt"
	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/go-kratos/kratos/v2/log"
	"net/url"
	"time"

	ws "github.com/gorilla/websocket"

	_ "github.com/go-kratos/kratos/v2/encoding/json"
	_ "github.com/go-kratos/kratos/v2/encoding/proto"
)

type ClientMessageHandler func(Message) error

type ClientHandlerData struct {
	Handler ClientMessageHandler
	Binder  Binder
}
type ClientMessageHandlerMap map[MessageCmd]*ClientHandlerData

type Client struct {
	conn *ws.Conn

	url      string
	endpoint *url.URL

	codec           encoding.Codec
	messageHandlers ClientMessageHandlerMap

	timeout time.Duration

	msgType MsgType
	log     *log.Helper
}

func NewClient(opts ...ClientOption) *Client {
	cli := &Client{
		url:             "",
		timeout:         1 * time.Second,
		codec:           encoding.GetCodec("json"),
		messageHandlers: make(ClientMessageHandlerMap),
		msgType:         MsgTypeBinary,
		log:             log.NewHelper(loggerName, log.DefaultLogger),
	}

	cli.init(opts...)

	return cli
}

func (c *Client) init(opts ...ClientOption) {
	for _, o := range opts {
		o(c)
	}

	c.endpoint, _ = url.Parse(c.url)
}

func (c *Client) Connect() error {
	if c.endpoint == nil {
		return errors.New("endpoint is nil")
	}

	c.log.Infof("connecting to %s", c.endpoint.String())

	conn, resp, err := ws.DefaultDialer.Dial(c.endpoint.String(), nil)
	if err != nil {
		c.log.Errorf("%s [%v]", err.Error(), resp)
		return err
	}
	c.conn = conn

	go c.run()

	return nil
}

func (c *Client) Disconnect() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.log.Errorf("disconnect error: %s", err.Error())
		}
		c.conn = nil
	}
}

func (c *Client) RegisterMessageHandler(cmd MessageCmd, handler ClientMessageHandler, binder Binder) {
	if _, ok := c.messageHandlers[cmd]; ok {
		return
	}

	c.messageHandlers[cmd] = &ClientHandlerData{handler, binder}
}

func RegisterClientMessageHandler[T any](cli *Client, cmd MessageCmd, handler func(*T) error) {
	cli.RegisterMessageHandler(cmd,
		func(message Message) error {
			switch t := message.(type) {
			case *T:
				return handler(t)
			default:
				return errors.New("invalid payload struct type")
			}
		},
		func() Message {
			var t T
			return &t
		},
	)
}

func (c *Client) DeregisterMessageHandler(cmd MessageCmd) {
	delete(c.messageHandlers, cmd)
}

func (c *Client) SendMessage(message interface{}) error {
	buff, err := c.marshalMessage(message)
	if err != nil {
		return err
	}

	switch c.msgType {
	case MsgTypeBinary:
		if err = c.sendBinaryMessage(buff); err != nil {
			return err
		}
		break

	case MsgTypeText:
		if err = c.sendTextMessage(string(buff)); err != nil {
			return err
		}
		break
	}

	return nil
}

func (c *Client) sendPingMessage(message string) error {
	return c.conn.WriteMessage(ws.PingMessage, []byte(message))
}

func (c *Client) sendPongMessage(message string) error {
	return c.conn.WriteMessage(ws.PongMessage, []byte(message))
}

func (c *Client) sendTextMessage(message string) error {
	return c.conn.WriteMessage(ws.TextMessage, []byte(message))
}

func (c *Client) sendBinaryMessage(message []byte) error {
	return c.conn.WriteMessage(ws.BinaryMessage, message)
}

func (c *Client) run() {
	defer c.Disconnect()

	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseNormalClosure, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				c.log.Errorf("read message error: %v", err)
			}
			return
		}

		switch messageType {
		case ws.CloseMessage:
			return

		case ws.BinaryMessage:
			_ = c.messageHandler(data)
			break

		case ws.TextMessage:
			_ = c.messageHandler(data)
			break

		case ws.PingMessage:
			if err := c.sendPongMessage(""); err != nil {
				fmt.Println("write pong message error: ", err)
				return
			}
			break

		case ws.PongMessage:
			break
		}

	}
}

func (c *Client) marshalMessage(message Message) ([]byte, error) {
	var (
		codecJsonMsg,
		msgWithLength []byte
		err error
	)

	codecJsonMsg, err = CodecMarshal(c.codec, message)
	if err != nil {
		return nil, err
	}

	msgWithLength, err = LengthMarshal(codecJsonMsg, c.msgType)
	if err != nil {
		return nil, err
	}

	//LogInfo("msgWithLength:", string(msgWithLength))

	return msgWithLength, nil
}

func (c *Client) unmarshalMessage(msgWithLength []byte) (*ClientHandlerData, Message, error) {
	var (
		msgWithoutLength []byte
		length           uint32
		handler          *ClientHandlerData
		message          Message
		baseMsg          BaseMsg
		ok               bool
		err              error
	)

	msgWithoutLength, length, err = LengthUnmarshal(msgWithLength, c.msgType)
	if err != nil {
		return nil, nil, fmt.Errorf("lengthUnmarshal message exception:%v", err)
	}

	if int(length) != len(msgWithoutLength) {
		return nil, nil, fmt.Errorf("incomplete message")
	}

	err = CodecUnmarshal(c.codec, msgWithoutLength, &baseMsg)
	if err != nil {
		return nil, nil, fmt.Errorf("parse the Json command failed:%v", err)
	}

	handler, ok = c.messageHandlers[baseMsg.Command]
	if !ok {
		return nil, nil, errors.New("message handler not found")
	}

	if handler.Binder != nil {
		message = handler.Binder()
		err = CodecUnmarshal(c.codec, msgWithoutLength, &message)
		if err != nil {
			return nil, nil, fmt.Errorf("parse the Json failed:%v", err)
		}
	} else {
		return nil, nil, errors.New("message Binder not found")
	}

	return handler, message, nil
}

func (c *Client) messageHandler(buf []byte) error {
	var err error
	var handler *ClientHandlerData
	var message Message

	if handler, message, err = c.unmarshalMessage(buf); err != nil {
		return err
	}
	//LogDebug(payload)

	if err = handler.Handler(message); err != nil {
		return err
	}

	return nil
}
