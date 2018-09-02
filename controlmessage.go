package main

import (
	"encoding/hex"
	"log"
	"net"
	"sync"
	"time"
)

var (
	steeringBaseDataLock sync.Mutex
	// All data without checksum. Will be calculated before sending
	steeringBaseData, _  = hex.DecodeString("ff080000000090101000")
	stopData, _          = hex.DecodeString("ff087e3f403f901010a0")
	landData, _          = hex.DecodeString("ff087e3f403f90101080")
	controlsOnData, _    = hex.DecodeString("ff08003f403f10101000")
	hoverOnData, _       = hex.DecodeString("ff087e3f403f90101000")
	rotorOnData, _       = hex.DecodeString("ff087e3f403f90101040")
	gyroCalibData, _     = hex.DecodeString("ff08003f403f50101000")
	compassActiveData, _ = hex.DecodeString("ff087e3f403f90101010") // No idea what this is for
)

// adjustSteering will adjust the axes so the drone can stand as still as possible if not steered
// yaw = max 160, min 128, mid 144 - pitch = max 32, min 0, mid 16 - pitch = max 32, min 0, mid
func adjustSteering(yaw uint8, pitch uint8, roll uint8) {
	steeringBaseDataLock.Lock()
	steeringBaseData[6] = yaw
	steeringBaseData[7] = pitch
	steeringBaseData[8] = roll
	steeringBaseDataLock.Unlock()
}

func adjustSteeringRoll(totalValue uint8) {
	steeringBaseDataLock.Lock()
	steeringBaseData[8] = totalValue
	steeringBaseDataLock.Unlock()
}

func adjustSteeringPitch(totalValue uint8) {
	steeringBaseDataLock.Lock()
	steeringBaseData[7] = totalValue
	steeringBaseDataLock.Unlock()
}

// sendSteeringData steers the drone. uplift == up/down, yaw == turn, pitch == forward/backward, roll == sideways
func sendSteeringData(uplift float32, yaw float32, pitch float32, roll float32, conn net.Conn) {
	steeringBaseDataLock.Lock()
	// Fit -1 to 1 range into 0 to 255 range of drone
	steeringBaseData[2] = uint8((uplift + 1) * 127.5)
	steeringBaseData[3] = uint8(((yaw + 1) * 127.5) / 2)
	steeringBaseData[4] = uint8(((pitch + 1) * 127.5) / 2)
	steeringBaseData[5] = uint8(((roll + 1) * 127.5) / 2)
	sendMessage(steeringBaseData, conn)
	steeringBaseDataLock.Unlock()
}

func sendStopData(conn net.Conn) {
	log.Println("Sending stop data")
	sendMessageDuration(stopData, conn, 500*time.Millisecond)
}

func sendLandData(conn net.Conn) {
	log.Println("Sending stop data")
	sendMessageDuration(landData, conn, 500*time.Millisecond)
}

func sendMessageDuration(message []byte, conn net.Conn, duration time.Duration) {
	ticker := time.NewTicker(messageRate)
	defer ticker.Stop()
	done := make(chan bool)
	go func() {
		time.Sleep(duration)
		done <- true
	}()
	for {
		select {
		case <-done:
			// ticker ended
			return
		case <-ticker.C:
			sendMessage(message, conn)
		}
	}
}

func sendMessage(message []byte, conn net.Conn) {
	// Add checksum to byte slice (JJRC chose a simple algorithm: substract all bytes from each other)
	_, err := conn.Write(append(message, message[0]-message[1]-message[2]-message[3]-message[4]-message[5]-message[6]-message[7]-message[8]-message[9]))
	if err != nil {
		log.Println("Error while sending data: ", err)
	}
}
