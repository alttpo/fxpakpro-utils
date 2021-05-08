package main

import (
	"encoding/hex"
	"fmt"
	"github.com/aybabtme/uniplot/histogram"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

func main() {
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
		log.Println("Failed to open serial port at any baud rate")
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

	writeTest(f)
	//writeTestSpinLoop(f)
}

func writeTest(f serial.Port) {
	// Perform some timing tests:
	const expectedBytes = 0xFF
	const expectedPaddedBytes = 0x100
	sb := makeVGET(0xF50000, expectedBytes)

	log.Printf("VGET command:\n%s\n", hex.Dump(sb))

	const iterations = 600
	times := [iterations]float64{}

	start := time.Now()
	lastWrite := start
writeloop:
	for i := 0; i < iterations; i++ {
		// write:
		lastWrite = time.Now()
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
		data := readData(f, expectedPaddedBytes, expectedBytes)
		if data == nil {
			continue writeloop
		}
		//log.Printf("VGET response:\n%s\n", hex.Dump(data))
		log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])

		times[i] = float64(time.Now().Sub(lastWrite).Nanoseconds())
	}
	end := time.Now()

	log.Printf("%#v ns total; %#v ns avg\n", end.Sub(start).Nanoseconds(), end.Sub(start).Nanoseconds() / 600)
	hist := histogram.Hist(100, times[:])
	err := histogram.Fprintf(log.Writer(), hist, histogram.Linear(40), func(v float64) string {
		return time.Duration(v).String()
	})
	if err != nil {
		log.Println(err)
	}
}

func writeTestSpinLoop(f serial.Port) {
	runtime.LockOSThread()

	// SNES master clock ~= 1.89e9/88 Hz
	// SNES runs 1 scanline every 1364 master cycles
	// Frames are 262 scanlines in non-interlace mode (1 scanline takes 1360 clocks every other frame)
	// 1 frame takes 357366 master clocks
	const snes_frame_clocks = (261 * 1364) + 1362

	const snes_frame_nanoclocks = snes_frame_clocks * 1_000_000_000

	// 1 frame takes 0.016639356613757 seconds
	// 0.016639356613757 seconds = 16,639,356.613757 nanoseconds

	const snes_frame_time_nanoseconds_int = int(snes_frame_nanoclocks) / (int(1.89e9) / int(88))
	const snes_frame_time_nanoseconds_flt = snes_frame_nanoclocks / ((1.89e9) / 88)

	// Perform some timing tests:
	const expectedBytes = 0xFF
	const expectedPaddedBytes = 0x100
	sb := makeVGET(0xF50000, expectedBytes)

	log.Printf("VGET command:\n%s\n", hex.Dump(sb))

	// duration between VGETs:
	const dur = time.Nanosecond * time.Duration(snes_frame_time_nanoseconds_int)

	last_t := time.Now()

writeloop:
	for i := 0; i < 600; i++ {
		for time.Since(last_t) < dur {
		}
		t := time.Now()

		//log.Printf("delta %v ns; %d frames remaining\n", t.Sub(last_t).Nanoseconds(), 600 - frames)
		//fmt.Printf("%v\n", t.Sub(last_t).Nanoseconds())
		last_t = t

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
		data := readData(f, expectedPaddedBytes, expectedBytes)
		if data == nil {
			continue writeloop
		}
		log.Printf("VGET response:\n%s\n", hex.Dump(data))
		log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])
	}

	runtime.UnlockOSThread()
}

func writeTestSpinLoopMeasure(f serial.Port) {
	runtime.LockOSThread()

	// SNES master clock ~= 1.89e9/88 Hz
	// SNES runs 1 scanline every 1364 master cycles
	// Frames are 262 scanlines in non-interlace mode (1 scanline takes 1360 clocks every other frame)
	// 1 frame takes 357366 master clocks
	const snes_frame_clocks = (261 * 1364) + 1362

	const snes_frame_nanoclocks = snes_frame_clocks * 1_000_000_000

	// 1 frame takes 0.016639356613757 seconds
	// 0.016639356613757 seconds = 16,639,356.613757 nanoseconds

	const snes_frame_time_nanoseconds_int = int(snes_frame_nanoclocks) / (int(1.89e9) / int(88))
	const snes_frame_time_nanoseconds_flt = snes_frame_nanoclocks / ((1.89e9) / 88)

	// Perform some timing tests:
	const expectedBytes = 0xFF
	const expectedPaddedBytes = 0x100
	sb := makeVGET(0xF50000, expectedBytes)

	log.Printf("VGET command:\n%s\n", hex.Dump(sb))

	// duration between VGETs:
	const dur = time.Nanosecond * time.Duration(snes_frame_time_nanoseconds_int)

	last_t := time.Now()

writeloop:
	for i := 0; i < 600; i++ {
		for time.Since(last_t) < dur {
		}
		t := time.Now()

		//log.Printf("delta %v ns; %d frames remaining\n", t.Sub(last_t).Nanoseconds(), 600 - frames)
		//fmt.Printf("%v\n", t.Sub(last_t).Nanoseconds())
		last_t = t

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
		data := readData(f, expectedPaddedBytes, expectedBytes)
		if data == nil {
			continue writeloop
		}
		//log.Printf("VGET response:\n%s\n", hex.Dump(data))
		log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])
	}

	runtime.UnlockOSThread()
}

func readData(f serial.Port, expectedPaddedBytes int, expectedBytes int) (data []byte) {
	nr := 0
	data = make([]byte, 0, expectedPaddedBytes)
	for nr < expectedPaddedBytes {
		rb := make([]byte, 256)
		log.Printf("read()\n")
		n, err := f.Read(rb)
		if err != nil {
			log.Printf("read(): %v\n", err)
			return nil
		}

		nr += n
		log.Printf("read(): %d bytes (%d bytes total)\n", n, nr)

		data = append(data, rb[:n]...)
	}

	if nr != expectedPaddedBytes {
		log.Printf("read(): expected %d padded bytes but got %d\n", expectedPaddedBytes, nr)
	}

	data = data[:expectedBytes]
	return
}
