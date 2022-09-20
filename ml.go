package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"os/exec"
	"scrml/protos"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	MarkedImageSizePixels = 64
)

type MlStruct struct {
	mut    sync.Mutex
	cmd    *exec.Cmd
	conn   *grpc.ClientConn
	client *protos.MessagerClient
}

var (
	Ml MlStruct
)

func (m *MlStruct) Lock() {
	m.mut.Lock()
}

func (m *MlStruct) Unlock() {
	m.mut.Unlock()
}

func (m *MlStruct) ImageToArray(img *image.RGBA) *[]byte {
	ri := Image.Resize(img, MarkedImageSizePixels)

	b := make([]byte, 0, MarkedImageSizePixels*MarkedImageSizePixels*3)
	for i := 0; i < int(len(ri.Pix)); i += 4 {
		b = append(b, ri.Pix[i+0])
		b = append(b, ri.Pix[i+1])
		b = append(b, ri.Pix[i+2])
	}

	return &b
}

func (m *MlStruct) GRPCSendTrainingSampleData(b *[]byte, y byte) error {
	_, err := m.getClient().AppendTrainingSample(context.Background(), &protos.MsgSample{XData: *b, YData: int64(y)})
	if err != nil {
		return fmt.Errorf("append training sample error: %v", err)
	}
	return nil
}

func (m *MlStruct) GRPCTest() error {
	_, err := m.getClient().Test(context.Background(), &protos.VoidMsg{})
	if err != nil {
		return fmt.Errorf("append training sample error: %v", err)
	}
	return nil
}

func (m *MlStruct) GRPCSendPredictSampleData(b *[]byte) error {
	_, err := m.getClient().AppendPredictSample(context.Background(), &protos.MsgPredIn{Data: *b})
	if err != nil {
		return fmt.Errorf("append predict sample error: %v", err)
	}
	return nil
}

func (m *MlStruct) GRPCPredict() ([]byte, error) {
	data, err := m.getClient().Predict(context.Background(), &protos.VoidMsg{})
	if err != nil {
		return nil, fmt.Errorf("predict error: %v", err)
	}
	return data.Data, nil
}

func (m *MlStruct) GRPCInitMlParams() error {
	_, err := m.getClient().InitMlParams(context.Background(), &protos.MsgInit{SampleSize: MarkedImageSizePixels})
	if err != nil {
		return fmt.Errorf("init params error: %v", err)
	}
	return nil
}

func (m *MlStruct) GRPCTrain() error {
	_, err := m.getClient().Train(context.Background(), &protos.VoidMsg{})
	if err != nil {
		return fmt.Errorf("train error: %v", err)
	}
	return nil
}

func (m *MlStruct) StartMlServer() error {
	m.cmd = exec.Command("python", "mlserver.py")
	err := m.cmd.Start()
	if err != nil {
		log.Println("ML:", err)
		return err
	}
	return nil
}

func (m *MlStruct) StopMlServer() {
	m.cmd.Process.Kill()
}

func (m *MlStruct) ConnectServer() error {
	mlsrv_conn, err := grpc.Dial(":50555", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("MlServer connect error: %v", err)
	}
	m.conn = mlsrv_conn
	x := protos.NewMessagerClient(mlsrv_conn)
	m.client = &x
	return nil
}

func (m *MlStruct) ServerConnected() bool {
	if m.conn == nil {
		return false
	}
	return m.conn.GetState() == connectivity.Ready
}

func (m *MlStruct) DisconnectServer() {
	m.client = nil
	m.conn.Close()
	m.conn = nil
}

func (m *MlStruct) getClient() protos.MessagerClient {
	return (*m.client)
}
