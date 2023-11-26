package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

	speedTest(f)

	disableSram(f)

	//iovmTest1(f)
	//iovmTest2(f)

	speedTest(f)

	enableSram(f)
}

func readUntilTimeout(f serial.Port, cb func([]byte)) {
	rsp := [512]byte{}
	for {
		var err error
		var n int

		_ = f.SetReadTimeout(time.Microsecond * 16666 * 30)
		n, err = f.Read(rsp[:])
		if err != nil {
			log.Printf("read(): %v\n", err)
			return
		}

		if cb != nil {
			cb(rsp[:n])
		}
		//log.Printf("read: %d bytes\n%s\n", n, hex.Dump(rsp[:n]))

		if n == 0 {
			break
		}
	}
}

func readChunk(f serial.Port, chunk []byte) (err error) {
	n := 0
	ns := 0
	for ; ns < len(chunk); ns += n {
		n, err = f.Read(chunk[ns:])
		if err != nil {
			return fmt.Errorf("readChunk: error; read %d bytes: %w\n", n, err)
		}
	}
	if ns != len(chunk) {
		return fmt.Errorf("readChunk: expected to read %d bytes but read %d\n", len(chunk), n)
	}
	return nil
}

func write(f serial.Port, b []byte) (err error) {
	log.Printf("write: %d bytes\n%s\n", len(b), hex.Dump(b))
	_, err = f.Write(b)
	if err != nil {
		log.Printf("write(): %v\n", err)
		return
	}

	return
}

func disableSram(f serial.Port) {
	var sb [512]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpSRAM_ENABLE)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(0)
	// disable SRAM:
	sb[7] = 0

	log.Printf("disable SRAM writes\n")
	if write(f, sb[:]) != nil {
		return
	}
	readUntilTimeout(f, nil)
}

func enableSram(f serial.Port) {
	var sb [512]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpSRAM_ENABLE)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(0)
	// enable SRAM:
	sb[7] = 1

	log.Printf("enable SRAM writes\n")
	if write(f, sb[:]) != nil {
		return
	}
	readUntilTimeout(f, nil)
}

func iovmTest1(f serial.Port) {
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
	// read WRAM at [$7EF340] for 256 bytes:
	b = append(b, 0x00, 0x00, 0x40, 0xF3, 0x00, 0x00)
	// write to $2C00: `LDA #$04; STA $7EF359; STZ $2C00; JMP ($FFEA)`
	b = append(b, 0x01, 0x05, 0x00, 0x00, 0x00, 0x0C, 0xA9, 0x04, 0x8F, 0x59, 0xF3, 0x7E, 0x9C, 0x00, 0x2C, 0x6C, 0xEA, 0xFF)

	// set length of program:
	sb[7] = byte(len(b))

	if write(f, sb[:]) != nil {
		return
	}
	readUntilTimeout(f, func(rsp []byte) {
		log.Printf("read: %d bytes\n%s\n", len(rsp), hex.Dump(rsp))
	})
}

func iovmTest2(f serial.Port) {
	var sb [64]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpIOVM_EXEC)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B)

	b := sb[8:8:cap(sb)]
	// wait until WRAM[$F343] < 25:
	b = append(b, 0x0A, 0x00, 0x43, 0xF3, 0x00, 25, 0xFF)

	// set length of program:
	sb[7] = byte(len(b))

	if write(f, sb[:]) != nil {
		return
	}
	readUntilTimeout(f, func(rsp []byte) {
		log.Printf("read: %d bytes\n%s\n", len(rsp), hex.Dump(rsp))
	})
}

func speedTest(f serial.Port) {
	p := message.NewPrinter(language.AmericanEnglish)

	log.Printf("1000 iterations of speed test\n")

	var tmp [64]byte

	var sb [64]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpIOVM_EXEC)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B)
	// 0-byte VM program just to test baseline latency:
	sb[7] = 0

	const iterations = 1000
	times := [iterations]float64{}

	start := time.Now()
	lastWrite := start
	for i := 0; i < iterations; i++ {
		// write:
		lastWrite = time.Now()
		n, err := f.Write(sb[:])
		if err != nil {
			log.Printf("write(): %v\n", err)
			continue
		}
		// log.Printf("write(): wrote %d bytes\n", n)
		if n != len(sb[:]) {
			log.Printf("write(): expected to write 64 bytes but wrote %d\n", n)
			continue
		}

		// read:
		err = readChunk(f, tmp[:])
		if err != nil {
			log.Printf("readChunk(): %v\n", err)
			continue
		}
		//log.Printf("VGET response:\n%s\n", hex.Dump(data))
		//log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])

		times[i] = float64(time.Now().Sub(lastWrite).Nanoseconds())
	}

	//end := time.Now()
	//log.Printf("%#v ns total; %#v ns avg\n", end.Sub(start).Nanoseconds(), end.Sub(start).Nanoseconds() / iterations)

	reportHistograms(times[:], p)
}
