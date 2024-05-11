package websocket

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

var testClient *Client

func handleReqServiceRetMsg(message *ReqServiceRetMsg) error {
	fmt.Printf("收到消息: %v\n", message)

	//testServer.SendMessage(sessionId,message)

	return nil
}

func handleControlRetMsg(message *ControlRetMsg) error {
	fmt.Printf("message: %v\n", message)

	//testServer.SendMessage(sessionId,message)

	return nil
}

func handleNoticeMsg(message *NoticeMsg) error {
	fmt.Printf("message: %v\n", message)

	//testServer.SendMessage(sessionId,message)

	return nil
}

func sendMessage(msg interface{}) error {
	fmt.Printf("发送消息: %v\n", msg)
	return testClient.SendMessage(msg)
}

func TestClient(t *testing.T) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	cli := NewClient(
		WithEndpoint("ws://localhost:9081/openai"),
		WithClientCodec("json"),
		WithClientPayloadType(MsgTypeBinary),
	)
	defer cli.Disconnect()

	testClient = cli

	RegisterClientMessageHandler(cli, "reqServiceRet", handleReqServiceRetMsg)
	RegisterClientMessageHandler(cli, "controlRet", handleControlRetMsg)
	RegisterClientMessageHandler(cli, "notice", handleNoticeMsg)

	err := cli.Connect()
	if err != nil {
		t.Error(err)
	}

	<-interrupt
}

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func generateChallengeKey() (string, error) {
	p := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

func Test1(t *testing.T) {
	challengeKey, _ := generateChallengeKey()
	fmt.Println(computeAcceptKey(challengeKey))

	fmt.Println(computeAcceptKey("foIGUMVOg/QOba9qZkaCmg=="))
	fmt.Println(computeAcceptKey("UHF9V2jktxC//1zmwLnxMg=="))
	fmt.Println(computeAcceptKey("KWtssYGuj2uQiv7bG7tc7A=="))
	fmt.Println(computeAcceptKey("3G4O+cC9DDGJS9pJAhzpUA=="))
}
