package main

import (
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
)

func main() {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	portName := ""
	for _, port := range ports {
		if !port.IsUSB {
			continue
		}

		fmt.Printf("%s: Found USB port\n", port.Name)
		if port.IsUSB {
			fmt.Printf("   USB ID     %s:%s\n", port.VID, port.PID)
			fmt.Printf("   USB serial %s\n", port.SerialNumber)
		}

		if port.SerialNumber == "DEMO00000000" {
			portName = port.Name
			log.Printf("%s: FX Pak Pro found\n", portName)
			break
		}
	}

	if portName == "" {
		log.Fatal("No FX Pak Pro found\n")
	}

	// Try all the common baud rates in descending order:
	bauds := []int{
		921600,
		460800,
		256000,
		230400,
		153600,
		128000,
		115200,
		76800,
		57600,
		38400,
		28800,
		19200,
		14400,
		9600,
		7200,
		4800,
		2400,
		1800,
		1200,
		600,
		300,
		200,
		150,
		134,
		110,
		75,
		50,
	}

	f := serial.Port(nil)
	for _, baud := range bauds {
		log.Printf("%s: open(%d)\n", portName, baud)
		f, err = serial.Open(portName, &serial.Mode{
			BaudRate: baud,
			DataBits: 8,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		})
		if err == nil {
			break
		}
		log.Printf("%s: %v\n", portName, err)
	}
	if err != nil {
		log.Fatal("Failed to open serial port at any baud rate\n")
	}
	log.Printf("%s: success!\n", portName)

	// Close the port:
	defer (func() {
		log.Printf("%s: close()\n", portName)
		err = f.Close()
		if err != nil {
			log.Printf("%s: %v\n", portName, err)
		}
	})()

	// Perform some timing tests:
	sb := make([]byte, 64)
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpVGET)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B | FlagNORESP)
	// 4-byte struct: 1 byte size, 3 byte address
	sb[32] = byte(0xF0)
	addr := 0x7E0010
	sb[33] = byte((addr >> 16) & 0xFF)
	sb[34] = byte((addr >> 8) & 0xFF)
	sb[35] = byte((addr >> 0) & 0xFF)

	rb := make([]byte, 512)

	for i := 0; i < 600; i++ {
		// write:
		log.Printf("%s: write(VGET)\n", portName)
		n, err := f.Write(sb)
		if err != nil {
			log.Printf("%s: %v\n", portName, err)
			continue
		}
		if n != 64 {
			log.Printf("%s: expected to write 64 bytes but wrote %d\n", portName, n)
			continue
		}

		// read:
		n, err = f.Read(rb)
		rb = rb[:n]

	}
}
