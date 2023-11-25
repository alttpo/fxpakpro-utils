package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/aybabtme/uniplot/histogram"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"io"
	"log"
	"math"
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
	var sb [512]byte
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpIOVM_EXEC)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B)

	b := sb[8:8:512]
	// wait until [$2C00] & $FF == 0:
	b = append(b, 0x02, 0x05, 0x00, 0x00, 0x00, 0x00, 0xFF)
	// write to $2C00: LDA #$04; STA $7EF359; STZ $2C00; JMP ($FFEA)
	b = append(b, 0x01, 0x05, 0x00, 0x00, 0x00, 0x0C, 0xA9, 0x04, 0x8F, 0x59, 0xF3, 0x7E, 0x9C, 0x00, 0x2C, 0x6C, 0xEA, 0xFF)

	// set length of program:
	sb[7] = byte(len(b))

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

func writeGETTest(f serial.Port) {
	p := message.NewPrinter(language.AmericanEnglish)

	var tmp [0x2200]byte

	// Perform some timing tests:
	gatherSizes := [...]uint32{
		0x01 * 8, 0x02 * 8, 0x04 * 8, 0x08 * 8, 0x10 * 8, 0x20 * 8, 0x40 * 8, 0x80 * 8, 0xFF * 8,
		0x1000, 0x2000}
	for _, size := range gatherSizes {
		var sb [512]byte
		sb[0] = byte('U')
		sb[1] = byte('S')
		sb[2] = byte('B')
		sb[3] = byte('A')
		sb[4] = byte(OpGET)
		sb[5] = byte(SpaceSNES)
		sb[6] = byte(FlagNONE)

		addr := uint32(0xF50000)
		// size:
		sb[252] = byte((size >> 24) & 0xFF)
		sb[253] = byte((size >> 16) & 0xFF)
		sb[254] = byte((size >> 8) & 0xFF)
		sb[255] = byte((size >> 0) & 0xFF)
		// addr:
		sb[256] = byte((addr >> 24) & 0xFF)
		sb[257] = byte((addr >> 16) & 0xFF)
		sb[258] = byte((addr >> 8) & 0xFF)
		sb[259] = byte((addr >> 0) & 0xFF)

		expectedBytes := int(size)
		expectedPaddedBytes := (expectedBytes / 512) * 512
		if expectedBytes&511 != 0 {
			expectedPaddedBytes += 512
		}

		log.Printf("GET command:\n%s\n", hex.Dump(sb[:]))

		const iterations = 500
		times := [iterations]float64{}

		start := time.Now()
		lastWrite := start
		for i := 0; i < iterations; i++ {
			// write:
			lastWrite = time.Now()
			// log.Printf("write(GET)\n")
			n, err := f.Write(sb[:])
			if err != nil {
				log.Printf("write(): %v\n", err)
				continue
			}
			// log.Printf("write(): wrote %d bytes\n", n)
			if n != len(sb[:]) {
				log.Printf("write(): expected to write 512 bytes but wrote %d\n", n)
				continue
			}

			// response:
			var rsp [512]byte
			err = readChunk(f, rsp[:])
			if err != nil {
				log.Println(err)
				continue
			}

			// read:
			err = readChunk(f, tmp[:expectedPaddedBytes])
			if err != nil {
				log.Printf("readChunk(): %v\n", err)
				continue
			}
			//log.Printf("GET response:\n%s\n", hex.Dump(data))
			//log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])

			times[i] = float64(time.Now().Sub(lastWrite).Nanoseconds())
		}

		//end := time.Now()
		//log.Printf("%#v ns total; %#v ns avg\n", end.Sub(start).Nanoseconds(), end.Sub(start).Nanoseconds() / iterations)

		reportHistograms(times[:], p)
	}
}

func writeVGETTest(f serial.Port) {
	p := message.NewPrinter(language.AmericanEnglish)

	var tmp [0x2200]byte

	// Perform some timing tests:
	gatherSizes := [...]uint8{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80, 0xFF}
	for _, size := range gatherSizes {
		var sb [64]byte
		sb[0] = byte('U')
		sb[1] = byte('S')
		sb[2] = byte('B')
		sb[3] = byte('A')
		sb[4] = byte(OpVGET)
		sb[5] = byte(SpaceSNES)
		sb[6] = byte(FlagDATA64B | FlagNORESP)

		addr := uint32(0xF50000)
		expectedBytes := 0
		for i := 0; i < 8; i++ {
			sb[32+i*4] = byte(size)
			sb[33+i*4] = byte((addr >> 16) & 0xFF)
			sb[34+i*4] = byte((addr >> 8) & 0xFF)
			sb[35+i*4] = byte((addr >> 0) & 0xFF)
			addr += uint32(size)
			expectedBytes += int(size)
		}

		expectedPaddedBytes := (expectedBytes / 64) * 64
		if expectedBytes&0x3F != 0 {
			expectedPaddedBytes += 64
		}

		log.Printf("VGET command:\n%s\n", hex.Dump(sb[:]))

		const iterations = 500
		times := [iterations]float64{}

		start := time.Now()
		lastWrite := start
		for i := 0; i < iterations; i++ {
			// write:
			lastWrite = time.Now()
			// log.Printf("write(VGET)\n")
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
			err = readChunk(f, tmp[:expectedPaddedBytes])
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
}

func reportHistograms(times []float64, p *message.Printer) {
	cleaned, outliers := cleanData(times[:])

	hist := histogram.Hist(10, cleaned)
	err := histogram.Fprintf(log.Writer(), hist, histogram.Linear(40), func(v float64) string {
		return p.Sprintf("% 11dns", time.Duration(v).Nanoseconds())
		//return fmt.Sprintf("% 10dns", time.Duration(v).Nanoseconds())
		//return time.Duration(v).String()
	})
	if err != nil {
		log.Println(err)
	}

	fmt.Println()

	hist = histogram.Hist(10, outliers)
	err = histogram.Fprintf(log.Writer(), hist, histogram.Linear(40), func(v float64) string {
		return p.Sprintf("% 11dns", time.Duration(v).Nanoseconds())
		//return fmt.Sprintf("% 10dns", time.Duration(v).Nanoseconds())
		//return time.Duration(v).String()
	})
	if err != nil {
		log.Println(err)
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

func cleanData(a []float64) (cleaned []float64, outliers []float64) {
	cleaned = make([]float64, 0, len(a))
	for i := 1; i < len(a); i += 2 {
		if math.Abs(a[i]-a[i-1]) >= 10_000_000 {
			if a[i] > a[i-1] {
				outliers = append(outliers, a[i])
				cleaned = append(cleaned, a[i-1])
			} else {
				outliers = append(outliers, a[i-1])
				cleaned = append(cleaned, a[i])
			}
		} else {
			cleaned = append(cleaned, a[i-1], a[i])
		}
	}
	return
}
