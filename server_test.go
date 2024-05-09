package websocket

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

var testServer *Server
var i int

func handleConnect(sessionId SessionID, register bool) {
	if register {
		fmt.Printf("%s connected\n", sessionId)
	} else {
		fmt.Printf("%s disconnect\n", sessionId)
	}
}

func handleReqServiceMsg(sessionId SessionID, message *ReqServiceMsg) error {
	fmt.Printf("[%s] message: %v\n", sessionId, message)

	retMsg := ReqServiceRetMsg{BaseRetMsg: BaseRetMsg{BaseMsg: BaseMsg{message.Command + "Ret"}, RetCode: message.Payload.ID, RetMsg: "AS"}, Payload: struct {
		JobID   string `json:"JobID"`
		RunTime int    `json:"RunTime"`
		State   int    `json:"State"`
		Info    string `json:"Info"`
		Result  string `json:"Result"`
	}{JobID: "ASSA", RunTime: 1, State: 1, Info: "sa", Result: "qsqw"}}

	testServer.SendMessage(sessionId, retMsg)

	return nil
}

func handleControlMsg(sessionId SessionID, message *ControlMsg) error {
	fmt.Printf("[%s] message: %v\n", sessionId, message)

	//testServer.SendMessage(sessionId, message)

	return nil
}

func handleNoticeRetMsg(sessionId SessionID, message *NoticeRetMsg) error {
	fmt.Printf("[%s] message: %v\n", sessionId, message)

	//testServer.SendMessage(sessionId,message)

	return nil
}

func TestServer(t *testing.T) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	srv := NewServer(
		WithAddress(":10029"),
		WithPath("/"),
		WithConnectHandle(handleConnect),
		WithCodec("json"),
		WithMsgType(MsgTypeBinary),
	)

	RegisterServerMessageHandler(srv, "reqService", handleReqServiceMsg)
	RegisterServerMessageHandler(srv, "control", handleControlMsg)
	RegisterServerMessageHandler(srv, "noticeRet", handleNoticeRetMsg)

	testServer = srv

	if err := srv.Start(); err != nil {
		panic(err)
	}

	defer func() {
		if err := srv.Stop(); err != nil {
			t.Errorf("expected nil got %v", err)
		}
	}()

	<-interrupt
}

//func TestGob(t *testing.T) {
//	var msg BinaryMessage
//	msg.Type = MessageTypeChat
//	msg.Body = []byte("")
//
//	var buf bytes.Buffer
//	enc := gob.NewEncoder(&buf)
//	_ = enc.Encode(msg)
//
//	fmt.Printf("%s\n", string(buf.Bytes()))
//}
//
//func TestMessageMarshal(t *testing.T) {
//	var msg BinaryMessage
//	msg.Type = 10000
//	msg.Body = []byte("Hello World")
//
//	buf, err := msg.Marshal()
//	assert.Nil(t, err)
//
//	fmt.Printf("%s\n", string(buf))
//
//	var msg1 BinaryMessage
//	_ = msg1.Unmarshal(buf)
//
//	fmt.Printf("[%d] [%s]\n", msg1.Type, string(msg1.Body))
//}
