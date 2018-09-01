package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	messageRate = 25 * time.Millisecond

	stopControl = false

	globalController = &controller{directionY: 1} // Go up at first
)

type controller struct {
	directionX int // -1, 0, 1
	directionY int // -1, 0, 1
	directionZ int // -1, 0, 1
}

func (c *controller) SetDirectionX(direction int) {
	c.directionX = direction
}

func (c *controller) SetDirectionY(direction int) {
	c.directionY = direction
}

func (c *controller) SetDirectionZ(direction int) {
	c.directionZ = direction
}

func startControl() {
	// Create connection
	conn, err := net.Dial("udp", "172.16.10.1:8080")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Send Idle data
	log.Println("Sending prepare data")
	sendMessageDuration(gyroCalibData, conn, 2*time.Second)
	sendMessageDuration(controlsOnData, conn, 2*time.Second)
	sendMessageDuration(hoverOnData, conn, 2*time.Second)
	sendMessageDuration(rotorOnData, conn, 2*time.Second)

	// Catch SIGTERM
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// Send Stop data
		sendLandData(conn)
		log.Println("Exiting")
		os.Exit(1)
	}()

	for !stopControl {
		steer(conn)
		time.Sleep(messageRate)
	}
}

func steer(conn net.Conn) {
	uplift := float32(0)
	if globalController.directionY > 0 {
		uplift = 0.37
	} else if globalController.directionY < 0 {
		uplift = -0.3
	}
	roll := float32(0)
	if globalController.directionX > 0 {
		roll = 0.2
	} else if globalController.directionX < 0 {
		roll = -0.2
	}
	pitch := float32(0)
	if globalController.directionZ > 0 {
		pitch = 0.2
	} else if globalController.directionZ < 0 {
		pitch = -0.2
	}
	log.Println("uplift: ", uplift, "roll: ", roll, "pitch: ", pitch)
	sendSteeringData(uplift, 0, pitch, roll, conn)
}
