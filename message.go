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
}

type BaseRetMsg struct {
	BaseMsg
	RetCode int64  `json:"RetCode"`
	RetMsg  string `json:"RetMsg"`
}

// AI服务报文
type ReqServiceMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		Demand string `json:"Demand"`
		Train  struct {
			AlgoritmName    string `json:"AlgoritmName"`
			AlgoritmVersion string `json:"AlgoritmVersion"`
			ImageName       string `json:"ImageName"`
			ImageVersion    string `json:"ImageVersion"`
			DataSetPath     string `json:"DataSetPath"`
			Config          []struct {
				Command          string `json:"Command"`
				ResourceSpecName string `json:"ResourceSpecName"`
				Parameters       []struct {
					Key   string `json:"Key"`
					Value string `json:"Value"`
				} `json:"Parameters"`
				TaskNumber            int `json:"TaskNumber"`
				MinFailedTaskCount    int `json:"MinFailedTaskCount"`
				MinSucceededTaskCount int `json:"MinSucceededTaskCount"`
			} `json:"Config"`
		} `json:"Train"`
		Deduce struct {
			Service string `json:"Service"`
		} `json:"Deduce"`
		ResourcePool string `json:"ResourcePool"`
		UserName     string `json:"UserName"`
	} `json:"Payload"`
}

// AI服务报文应答
type ReqServiceRetMsg struct {
	BaseRetMsg
	Payload struct {
		Demand    string `json:"Demand"`
		TrainInfo struct {
			JobID  string `json:"JobID"`
			RunSec int64  `json:"RunSec"`
			State  int32  `json:"State"`
			Info   string `json:"Info"`
			Result string `json:"Result"`
		} `json:"TrainInfo"`
		DeduceInfo struct {
		} `json:"DeduceInfo"`
	} `json:"Payload"`
}

// AI服务控制报文
type ControlMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		JobID    string `json:"JobID"`
		Demand   string `json:"Demand"`
		UserName string `json:"UserName"`
	} `json:"Payload"`
}

// AI服务控制报文应答
type ControlRetMsg struct {
	BaseRetMsg
	Payload struct {
		Demand    string `json:"Demand"`
		JobID     string `json:"JobID"`
		QueryInfo struct {
			RunSec int64  `json:"RunSec"`
			State  int32  `json:"State"`
			Info   string `json:"Info"`
			Result string `json:"Result"`
		} `json:"QueryInfo"`
		CancelInfo struct {
			CancelAt int64 `json:"CancelAt"`
		} `json:"CancelInfo"`
	} `json:"Payload"`
}

type NoticeMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		Demand string `json:"Demand"`
		Train  struct {
			JobID  string `json:"JobId"`
			RunSec int64  `json:"RunSec"`
			State  int32  `json:"State"`
			Info   string `json:"Info"`
			Result string `json:"Result"`
		} `json:"Train"`
		Minio struct {
			EndPoint        string `json:"EndPoint"`
			AccessKeyID     string `json:"AccessKeyID"`
			SecretAccessKey string `json:"SecretAccessKey"`
		} `json:"Minio"`
	} `json:"Payload"`
}

type NoticeRetMsg struct {
	BaseRetMsg
	Payload struct {
		Demand string `json:"Demand"`
	} `json:"Payload"`
}

type InfoMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		User string `json:"User"`
		Name string `json:"Name"`
		Pass string `json:"Pass"`
		Type string `json:"Type"`
		Size string `json:"Size"`
		Tel  string `json:"Tel"`
		UID  string `json:"UID"`
		NID  string `json:"NID"`
	} `json:"Payload"`
}

type InfoRetMsg struct {
	BaseRetMsg
}

type QueryMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		Type      string `json:"Type"`
		TimeStamp int64  `json:"TimeStamp"`
		NID       string `json:"NID"`
	} `json:"Payload"`
}

type Element struct {
	Count        int64  `json:"Count"`
	Price        int64  `json:"Price"`
	TimeStamp    int64  `json:"TimeStamp"`
	Url          string `json:"Url"`
	Introduction string `json:"Introduction"`
	Mode         int32  `json:"Mode"`
}

type Resource struct {
	Count     int64 `json:"Count"`
	Price     int64 `json:"Price"`
	TimeStamp int64 `json:"TimeStamp"`
	Mode      int32 `json:"Mode"`
}

type QueryRetMsg struct {
	BaseRetMsg
	Payload struct {
		Type        string `json:"Type"`
		Number      int    `json:"Number"`
		NID         string `json:"NID"`
		Description string `json:"Description"`
		FreeGPU     int    `json:"FreeGPU"`
		GPU         int    `json:"GPU"`
		Contact     string `json:"Contact"`
		Information string `json:"Information"`
		Content     struct {
			Element  []Element  `json:"Element"`
			Resource []Resource `json:"Resource"`
			Task     []Task     `json:"Task"`
			Node     []Node     `json:"Node"`
		} `json:"Content"`
	} `json:"Payload"`
}

type ElementMsg struct {
	BaseMsg
	ID      int64 `json:"ID"`
	Payload struct {
		Type        string    `json:"Type"`
		Number      int       `json:"Number"`
		NID         string    `json:"NID"`
		Contact     string    `json:"Contact"`
		Information string    `json:"Information"`
		Content     []Element `json:"Content"`
	} `json:"Payload"`
}

type ElementRetMsg struct {
	BaseRetMsg
}

type Task struct {
	TID         string `json:"TID"`
	UID         string `json:"UID"`
	State       int32  `json:"State"`
	Timestamp   int64  `json:"Timestamp"`
	Description string `json:"Description"`
}
type TaskMsg struct {
	Command string `json:"Command"`
	ID      int64  `json:"ID"`
	Payload struct {
		NID     string `json:"NID"`
		Content []Task `json:"Content"`
	} `json:"Payload"`
}

type TaskRetMsg struct {
	BaseRetMsg
}

type Node struct {
	Device   string `json:"Device"`
	CPU      int    `json:"CPU"`
	Memory   int    `json:"Memory"`
	Resource string `json:"Resource"`
}
type NodeMsg struct {
	Command string `json:"Command"`
	ID      int64  `json:"ID"`
	Payload struct {
		Number      int    `json:"Number"`
		NID         string `json:"NID"`
		Description string `json:"Description"`
		Content     []Node `json:"Content"`
	} `json:"Payload"`
}

type NodeRetMsg struct {
	BaseRetMsg
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
