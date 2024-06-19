package websocket

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/encoding"
	_ "github.com/go-kratos/kratos/v2/encoding/json"
	_ "github.com/go-kratos/kratos/v2/encoding/proto"
)

type Any interface{}
type MessageCmd string
type Message Any

type BaseMsg struct {
	Command MessageCmd `json:"Command"`
	ID      int64      `json:"ID"`
}

type BaseRetMsg struct {
	BaseMsg
	RetCode int64  `json:"RetCode"`
	RetMsg  string `json:"RetMsg"`
}

func CodecMarshal(codec encoding.Codec, msg Any) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}

	if codec != nil {
		dataBuffer, err := codec.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return dataBuffer, nil
	} else {
		switch t := msg.(type) {
		case []byte:
			return t, nil
		case string:
			return []byte(t), nil
		default:
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			if err := enc.Encode(msg); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
}

func CodecUnmarshal(codec encoding.Codec, inputData []byte, outValue interface{}) error {
	if codec != nil {
		if err := codec.Unmarshal(inputData, outValue); err != nil {
			return err
		}
	} else if outValue == nil {
		outValue = inputData
	}
	return nil
}

func LengthMarshal(OriginalMsg []byte, msgType MsgType) ([]byte, error) {
	buf := new(bytes.Buffer)
	switch msgType {
	case MsgTypeBinary:
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(OriginalMsg))); err != nil {
			return nil, err
		}
		buf.Write(OriginalMsg)
	case MsgTypeText:
		lengthStr := fmt.Sprintf("%04d", len(OriginalMsg))
		buf.Write([]byte(lengthStr))
		buf.Write(OriginalMsg)
	default:
		return nil, errors.New("invalid msg type")
	}

	return buf.Bytes(), nil
}

func LengthUnmarshal(MsgWithLength []byte, msgType MsgType) ([]byte, uint32, error) {
	var length32 uint32

	network := new(bytes.Buffer)

	switch msgType {
	case MsgTypeBinary:
		network.Write(MsgWithLength)

		if err := binary.Read(network, binary.LittleEndian, &length32); err != nil {
			return nil, 0, err
		}
	case MsgTypeText:
		lengthStr := string(network.Next(4))
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return nil, 0, err
		}
		length32 = uint32(length)
	default:
		return nil, 0, errors.New("invalid msg type")
	}

	return network.Bytes(), length32, nil
}
