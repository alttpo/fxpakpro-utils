package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	timestamp := strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05.000000"), ".", "-")
	logfilename := timestamp + ".txt"
	logfile, err := os.OpenFile(
		logfilename,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644)
	if err != nil {
		log.Println(err)
		return
	}
	defer (func() {
		logfile.Close()
		fmt.Printf("Output written to '%s'\n", logfilename)
	})()
	log.SetOutput(io.MultiWriter(logfile, os.Stdout))

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		log.Println(err)
		return
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
		log.Println("No FX Pak Pro found")
		return
	}

	f := serial.Port(nil)
	log.Printf("%s: open(%d)\n", portName, 9600)
	f, err = serial.Open(portName, &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		log.Println("Failed to open serial port")
		log.Printf("%s: %v\n", portName, err)
		return
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

	// Disable GC
	debug.SetGCPercent(-1)

	iovmTest(f)
}

func iovmTest(f serial.Port) {
	var sb [64]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpIOVM_EXEC)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B)

	b := sb[8:8:cap(sb)]
	// wait until [$2C00] & $FF == 0:
	b = append(b, 0x02, 0x05, 0x00, 0x00, 0x00, 0x00, 0xFF)
	// write to $2C00: LDA #$04; STA $7EF359; STZ $2C00; JMP ($FFEA)
	b = append(b, 0x01, 0x05, 0x00, 0x00, 0x00, 0x0C, 0xA9, 0x04, 0x8F, 0x59, 0xF3, 0x7E, 0x9C, 0x00, 0x2C, 0x6C, 0xEA, 0xFF)

	// set length of program:
	sb[7] = byte(len(b))

	log.Printf("send:\n%s\n", hex.Dump(sb[:]))
	_, err := f.Write(sb[:])
	if err != nil {
		log.Printf("write(): %v\n", err)
		return
	}

	fullrsp := make([]byte, 0, 4096)
	rsp := [512]byte{}
	for {
		var n int
		f.SetReadTimeout(time.Second)
		n, err = f.Read(rsp[:])
		if err != nil {
			log.Printf("write(): %v\n", err)
			return
		}

		log.Printf("chunk:\n%s\n", hex.Dump(rsp[:n]))
		if n == 0 {
			break
		}

		fullrsp = append(fullrsp, rsp[:n]...)
	}

	log.Printf("full:\n%s\n", hex.Dump(fullrsp))
}
