package modbus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

type tcpServer struct {
	localAddress string
	slaves       map[byte][]float64
}

var dataLock = sync.Mutex{}

func netRead(conn net.Conn, readNum int, timeout time.Duration) (result []byte, err error) {
	//err = conn.SetReadDeadline(time.Now().Add(timeout))
	//if err != nil {
	//	return
	//}
	for {
		bytesToRead := readNum - len(result)
		if bytesToRead == 0 {
			return
		}
		tmp := make([]byte, bytesToRead)
		var bytesRead int
		bytesRead, err = conn.Read(tmp)
		result = append(result, tmp[:bytesRead]...)
		if err != nil {
			return
		}
	}
}

func netWrite(conn net.Conn, data []byte) error {
	fmt.Print("write:")
	fmt.Printf("%+x  ", data)
	fmt.Println()
	bytesToWite := 0
	for {
		if bytesToWite == len(data) {
			return nil
		}
		bytesWrited, err := conn.Write(data[bytesToWite:])
		if err != nil {
			return err
		}
		bytesToWite = bytesToWite + bytesWrited
	}
}

func code(v interface{}) []byte {
	buf := bytes.NewBuffer([]byte{})
	binary.Write(buf, binary.BigEndian, v)
	return buf.Bytes()
}

func decodeUint16(data []byte) uint16 {
	var x uint16
	binary.Read(bytes.NewBuffer(data), binary.BigEndian, &x)
	return x
}

func decodeUint8(data []byte) uint8 {
	var x uint8
	binary.Read(bytes.NewBuffer(data), binary.BigEndian, &x)
	return x
}

func readModbusTcp(conn net.Conn) ([]byte, error) {
	data, err := netRead(conn, 6, 3*time.Second)
	if err != nil {
		return data, err
	}
	readlen := decodeUint16(data[4:6])
	fmt.Println("readlen:", readlen)
	data2, err := netRead(conn, int(readlen), 3*time.Second)
	data = append(data, data2...)
	if err != nil {
		return data, err
	}
	fmt.Print("read:")
	fmt.Printf("%+x", data)
	fmt.Println()
	return data, nil
}

func (ts tcpServer) handle(conn net.Conn) error {
	for {
		readFrame, err := readModbusTcp(conn)
		if err != nil {
			fmt.Println("connection break", err)
			return err
		}
		slaveID := decodeUint8(readFrame[6:7])
		funcID := decodeUint8(readFrame[7:8])
		startAddr := decodeUint16(readFrame[8:10])
		number := decodeUint16(readFrame[10:12])
		fmt.Printf("slaveID:%d,funcID:%d,startAddr:%d,number:%d\n", slaveID, funcID, startAddr, number)
		dataLock.Lock()
		locallen := uint16(len(ts.slaves[slaveID]) * 4)
		var errCode = byte(0)
		if funcID != 3 {
			errCode = 1
		} else if startAddr+number > locallen {
			errCode = 2
		}
		if errCode != 0 {
			respondFrame := append([]byte{}, readFrame[0:4]...)
			respondFrame = append(respondFrame, code(uint16(3))...)
			respondFrame = append(respondFrame, byte(slaveID), byte(funcID)+0x80, errCode)
			netWrite(conn, respondFrame)
		} else {
			values := ts.slaves[slaveID]
			data := code(values)
			fmt.Println(data)
			data = data[(startAddr)*2 : (startAddr+number)*2]
			dataLen := len(data)
			respondFrame := append([]byte{}, readFrame[0:4]...)
			respondFrame = append(respondFrame, code(uint16(dataLen+3))...)
			respondFrame = append(respondFrame, byte(slaveID), byte(funcID), byte(dataLen))
			respondFrame = append(respondFrame, data...)
			netWrite(conn, respondFrame)
		}
		dataLock.Unlock()
	}
}

func (ts tcpServer) Listen() error {
	fmt.Println("liseten on " + ts.localAddress)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", ts.localAddress)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go ts.handle(conn)
	}
}

func (ts *tcpServer) UpdateData(slaves map[byte][]float64) {
	dataLock.Lock()
	defer dataLock.Unlock()
	ts.slaves = slaves
}

func NewTCPServer(localAddress string) tcpServer {
	return tcpServer{localAddress: localAddress}
}
