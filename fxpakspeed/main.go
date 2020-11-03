package main

import (
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)

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
	const expectedBytes = 0xF0
	const expectedPaddedBytes = 0x100
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

writeloop:
	for i := 0; i < 600; i++ {
		// write:
		log.Printf("%s: write(VGET)\n", portName)
		n, err := f.Write(sb)
		if err != nil {
			log.Printf("%s: write(): %v\n", portName, err)
			continue
		}
		log.Printf("%s: write(): wrote %d bytes\n", portName, n)
		if n != len(sb) {
			log.Printf("%s: write(): expected to write 64 bytes but wrote %d\n", portName, n)
			continue
		}

		// read:
		nr := 0
		data := make([]byte, 0, expectedPaddedBytes)
		for nr < expectedPaddedBytes {
			rb := make([]byte, 64)
			log.Printf("%s: read()\n", portName)
			n, err = f.Read(rb)
			if err != nil {
				log.Printf("%s: read(): %v\n", portName, err)
				continue writeloop
			}

			nr += n
			log.Printf("%s: read(): %d bytes (%d bytes total)\n", portName, n, nr)

			data = append(data, rb[:n]...)
		}

		if nr != expectedPaddedBytes {
			log.Printf("%s: read(): expected %d padded bytes but got %d\n", portName, expectedPaddedBytes, nr)
		}

		data = data[:expectedBytes]
		log.Printf("%s: [$10] = $%02x; [$A0] = $%02x\n", portName, data[0x00], data[0x90])
	}
}
