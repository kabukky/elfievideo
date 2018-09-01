package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"gocv.io/x/gocv"
)

var (
	h264KeyframeData, _ = hex.DecodeString("0000000167")
	videoStartData, _   = hex.DecodeString("000102030405060708092828")
	frameTime           = 40 * time.Millisecond // 25 FPS (40 Milliseconds/Frame)
	videoCaptureStarted = false
)

type WriteChecker struct {
	WritingStarted bool
	Writer         io.Writer
}

func (wc *WriteChecker) Write(p []byte) (int, error) {
	if wc.WritingStarted {
		return wc.Writer.Write(p)
	}
	n := len(p)
	// Check if bytes contain key frame
	keyFrameIndex := bytes.Index(p, h264KeyframeData)
	if keyFrameIndex == -1 {
		// No keyframe yet. Do not write
		return n, nil
	}
	log.Println("Starting to write file")
	wc.WritingStarted = true
	_, err := wc.Write(p[keyFrameIndex:])
	videoCaptureStarted = true
	return n, err
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
	go startControl()

	// Wait for capture to file to start
	for {
		if videoCaptureStarted {
			break
		}
		time.Sleep(frameTime) // Sleep for a frame and try again
	}

	// Start gocv
	time.Sleep(frameTime * 4) // Sleep for a few frames so we have a small buffer
	videoCap, err := gocv.VideoCaptureFile("videotest.h264")
	if err != nil {
		panic(err)
	}
	defer videoCap.Close()
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
	// showVideo(videoCap, window)
	trackOpticalFlow(videoCap, window)
	// trackObject(videoCap, window)
	// TODO: Capture SIGINT
}
