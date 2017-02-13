package main

import (
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"bytes"

	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
)

var jpegImage image.Image
var jpegImage2 image.Image

func main() {
	file, _ := os.Open("loading.jpg")
	var err error
	jpegImage, _, err = image.Decode(file)
	if err != nil {
		log.Panic(err)
	}

	//log.Println(formatname)

	if jpegImage == nil {
		log.Panic("image was empty wtf")
	}

	go streamer()
	wde.Run()
}

func streamer() {
	var wg sync.WaitGroup

	x := func() {
		tw, err := wde.NewWindow(400, 240)
		if err != nil {
			fmt.Println(err)
			return
		}
		tw.SetTitle("Top")
		tw.SetSize(400, 240)
		tw.Show()

		bw, err := wde.NewWindow(320, 240)
		if err != nil {
			fmt.Println(err)
			return
		}
		bw.SetTitle("Bottom")
		bw.SetSize(320, 240)
		bw.Show()

		tcpAddr, err := net.ResolveTCPAddr("tcp4", "10.42.42.136:8000")
		checkError(err)

		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		checkError(err)

		data, err := hex.DecodeString(generateMessage())

		_, err = conn.Write(data)
		checkError(err)

		conn.Close()

		fmt.Println("Test")

		time.Sleep(time.Second * 3)

		conn, err = net.DialTCP("tcp", nil, tcpAddr)
		checkError(err)
		conn.Close()

		go videoServer()

		for {
			s := tw.Screen()
			if jpegImage != nil {
				draw.Draw(s, s.Bounds(), jpegImage, image.Point{0, 0}, draw.Src)
			}
			tw.FlushImage()

			s = bw.Screen()
			if jpegImage2 != nil {
				draw.Draw(s, s.Bounds(), jpegImage2, image.Point{0, 0}, draw.Src)
			}
			bw.FlushImage()
		}
	}

	wg.Add(1)
	go x()

	wg.Wait()
	wde.Stop()
}

func videoServer() {
	serverAddr, err := net.ResolveUDPAddr("udp", ":8001")
	checkError(err)

	serverConn, err := net.ListenUDP("udp", serverAddr)
	checkError(err)
	defer serverConn.Close()

	buf := make([]byte, 2000)

	priorityScreenBuffer := []byte{}
	secondaryScreenBuffer := []byte{}

	priorityExpectedFrame := byte(0)
	secondaryExpectedFrame := byte(0)

	priorityExpectedPacket := byte(0)
	secondaryExpectedPacket := byte(0)

	activePriorityMode := byte(1)

	for {
		n, _, err := serverConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error: ", err)
			continue
		}

		// We don't have enough for a header here so throw this trash away
		if n < 4 {
			continue
		}

		currentFrame := buf[0]
		currentScreen := buf[1] & 0x0F
		isLastPacket := (buf[1] & 0xF0) >> 4
		currentPacket := buf[3]

		if priorityExpectedFrame == 0 && currentScreen == activePriorityMode {
			priorityExpectedFrame = currentFrame
		} else if secondaryExpectedFrame == 0 {
			secondaryExpectedFrame = currentFrame
		}

		if priorityExpectedFrame == currentFrame && priorityExpectedPacket == currentPacket && activePriorityMode == currentScreen {
			//priority screen
			priorityScreenBuffer = append(priorityScreenBuffer, buf[4:n]...)
			priorityExpectedPacket++

			if isLastPacket == 1 {
				//await TryDisplayImage(priorityScreenBuffer, currentScreen);

				img, _, err := image.Decode(bytes.NewReader(priorityScreenBuffer))
				if err == nil && img != nil {
					jpegImage = img
				}

				priorityExpectedFrame = 0
				priorityExpectedPacket = 0
			}
		} else if currentScreen == activePriorityMode {
			//Priority Packet Dropped (unexpected packet or frame)
			priorityScreenBuffer = []byte{}
			priorityExpectedFrame = 0
			priorityExpectedPacket = 0

			continue
		} else if secondaryExpectedPacket == currentPacket {
			//secondary screen
			secondaryScreenBuffer = append(secondaryScreenBuffer, buf[4:n]...)
			secondaryExpectedPacket++

			if isLastPacket == 1 {
				img, _, err := image.Decode(bytes.NewReader(secondaryScreenBuffer))
				if err == nil && img != nil {
					jpegImage2 = img
				}

				secondaryExpectedFrame = 0
				secondaryExpectedPacket = 0
			}
			continue
		} else {
			//Secondary Packet Dropped (unexpected packet or frame)
			secondaryScreenBuffer = []byte{}
			secondaryExpectedFrame = 0
			secondaryExpectedPacket = 0
		}
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func generateMessage() string {
	message := fmt.Sprintf("78563412B80B00000000000085030000%02X%02X0000%02X0000000000%02X%0114d", 1, 1, 90, 2*101, 0)

	return message
}
