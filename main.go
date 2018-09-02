package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"gocv.io/x/gocv"

	"github.com/kabukky/h264"
)

var (
	h264KeyframeStartData, _ = hex.DecodeString("0000000167")
	h264KeyframeData, _      = hex.DecodeString("00000001")
	videoStartData, _        = hex.DecodeString("000102030405060708092828")
	frameTime                = 40 * time.Millisecond // 25 FPS (40 Milliseconds/Frame)
	videoCaptureStarted      = false

	currentFrameLock sync.RWMutex
	currentFrame     *gocv.Mat
)

type WriteChecker struct {
	Writer  io.Writer
	Decoder *h264.Decoder
	Buffer  bytes.Buffer
}

func (wc *WriteChecker) Write(p []byte) (int, error) {
	if wc.Decoder != nil {
		lenP, err := wc.Buffer.Write(p)
		if err != nil {
			panic(err)
		}
		nals := splitNals(wc.Buffer.Bytes())
		log.Println("Len nals: ", len(nals))
		if len(nals) > 1 {
			// leave last nal for next write
			for i := 0; i < len(nals)-1; i++ {
				img, err := wc.Decoder.Decode(nals[i])
				if err == nil {
					// log.Println("Bounds: ", img.Bounds())
					newFrame, err := gocv.ImageToMatRGBA(img)
					if err != nil {
						panic(err)
					}
					currentFrameLock.Lock()
					currentFrame = &newFrame
					currentFrameLock.Unlock()
					// Got newest image
					// newFrame, err := gocv.ImageToMatRGB(img)
					// if err == nil {
					// 	currentFrameLock.Lock()
					// 	currentFrame = &newFrame
					// 	currentFrameLock.Unlock()
					// } else {
					// 	panic(err)
					// }
				} else {
					log.Println("\n\n\n------------------\nDECODE FAILED!\n------------\n\n")
				}
			}
		}
		wc.Buffer.Reset()
		wc.Buffer.Write(nals[len(nals)-1])
		return lenP, nil
		// return wc.Writer.Write(p)
	}
	lenP := len(p)
	// Check if bytes contain key frame
	keyFrameStartIndex := bytes.Index(p, h264KeyframeStartData)
	if keyFrameStartIndex == -1 {
		// No keyframe yet. Do not write
		return lenP, nil
	}
	log.Println("Starting to write h264")
	nals := splitNals(p[keyFrameStartIndex:])
	log.Println("First nals:")
	for index := range nals {
		log.Println("\n" + hex.Dump(nals[index]))
	}
	var err error
	wc.Decoder, err = h264.NewDecoder(nals[0])
	if err != nil {
		panic(err)
	}
	// Write rest to buffer
	for index := range nals[1:] {
		_, err = wc.Buffer.Write(nals[index])
		if err != nil {
			panic(err)
		}
	}
	return lenP, err
}

func splitNals(input []byte) [][]byte {
	var nals [][]byte
	for i, elem := range bytes.Split(input, h264KeyframeData) {
		if i == 0 && len(elem) == 0 {
			// The first bytes were h264KeyframeData. Skip this iteration and append h264KeyframeData to the next element
			continue
		}
		nals = append(nals, append(h264KeyframeData, elem...))
	}
	return nals
}

func main() {
	// Needed for gocv
	runtime.LockOSThread()

	// Create connection
	conn, err := net.Dial("tcp", "172.16.10.1:8888")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	videoFile, err := os.Create("videotest.h264")
	if err != nil {
		panic(err)
	}
	defer videoFile.Close()
	writeChecker := &WriteChecker{Writer: videoFile}
	go func() {
		_, err := io.Copy(writeChecker, conn)
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		for {
			_, err = conn.Write(videoStartData)
			if err != nil {
				panic(err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Start control
	// go startControl()

	// Wait for capture to file to start
	for {
		if currentFrame != nil {
			break
		}
		time.Sleep(frameTime) // Sleep for a frame and try again
	}

	log.Println("Video capture started!")

	// Start gocv
	// time.Sleep(frameTime * 1) // Sleep for a few frames so we have a small buffer
	// videoCap, err := gocv.VideoCaptureFile("videotest.h264")
	// if err != nil {
	// 	panic(err)
	// }
	// defer videoCap.Close()
	window := gocv.NewWindow("Hello")
	defer window.Close()
	// Recover from panics in main goroutine (gocv) and land the drone safely
	defer func() {
		if err := recover(); err != nil {
			log.Println("Panic while running gocv: ", err)
			stopControl = true
			sendLandData(conn)
		}
	}()
	showVideo(window)
	// trackOpticalFlow(videoCap, window)
	// trackObject(videoCap, window)
	// TODO: Capture SIGINT
}
