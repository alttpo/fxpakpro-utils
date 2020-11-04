package main

import (
	"encoding/hex"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	timestamp := strings.ReplaceAll(time.Now().Format("2006-01-02T15-04-05.999999999"), ".", "-")
	logfile, err := os.OpenFile(
		timestamp + ".txt",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer logfile.Close()
	log.SetOutput(logfile)

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Fatal(err)
	}

	portName := ""
	for _, port := range ports {
		if !port.IsUSB {
			continue
		}

		log.Printf("%s: Found USB port\n", port.Name)
		if port.IsUSB {
			log.Printf("   USB ID     %s:%s\n", port.VID, port.PID)
			log.Printf("   USB serial %s\n", port.SerialNumber)
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

	log.Printf("%s: Set DTR on\n", portName)
	if err = f.SetDTR(true); err != nil {
		log.Printf("%s: %v\n", portName, err)
	}

	// Close the port:
	defer (func() {
		log.Printf("%s: Set DTR off\n", portName)
		if err = f.SetDTR(false); err != nil {
			log.Printf("%s: %v\n", portName, err)
		}
		log.Printf("%s: close()\n", portName)
		err = f.Close()
		if err != nil {
			log.Printf("%s: %v\n", portName, err)
		}
	})()

	// Perform some timing tests:
	const expectedBytes = 0xFF
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
	sb[32] = byte(expectedBytes)
	addr := 0xF50000
	sb[33] = byte((addr >> 16) & 0xFF)
	sb[34] = byte((addr >> 8) & 0xFF)
	sb[35] = byte((addr >> 0) & 0xFF)

	log.Printf("VGET command:\n%s\n", hex.Dump(sb))

writeloop:
	for i := 0; i < 600; i++ {
		// write:
		log.Printf("write(VGET)\n")
		n, err := f.Write(sb)
		if err != nil {
			log.Printf("write(): %v\n", err)
			continue
		}
		log.Printf("write(): wrote %d bytes\n", n)
		if n != len(sb) {
			log.Printf("write(): expected to write 64 bytes but wrote %d\n", n)
			continue
		}

		// read:
		nr := 0
		data := make([]byte, 0, expectedPaddedBytes)
		for nr < expectedPaddedBytes {
			rb := make([]byte, 256)
			log.Printf("read()\n")
			n, err = f.Read(rb)
			if err != nil {
				log.Printf("read(): %v\n", err)
				continue writeloop
			}

			nr += n
			log.Printf("read(): %d bytes (%d bytes total)\n", n, nr)

			data = append(data, rb[:n]...)
		}

		if nr != expectedPaddedBytes {
			log.Printf("read(): expected %d padded bytes but got %d\n", expectedPaddedBytes, nr)
		}

		data = data[:expectedBytes]
		log.Printf("VGET response:\n%s\n", hex.Dump(data))
		log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])
	}
}
