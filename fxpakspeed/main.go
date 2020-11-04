package main

import (
	"encoding/hex"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	timestamp := strings.ReplaceAll(time.Now().UTC().Format("2006-01-02T15-04-05.000000"), ".", "-")
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

	//writeTest(f)
	writeTestSpinLoop(f)
}

func writeTest(f serial.Port) {
	// Perform some timing tests:
	const expectedBytes = 0xFF
	const expectedPaddedBytes = 0x100
	sb := makeVGET(0xF50000, expectedBytes)

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
		data := readData(f, expectedPaddedBytes, expectedBytes)
		if data == nil {
			continue writeloop
		}
		log.Printf("VGET response:\n%s\n", hex.Dump(data))
		log.Printf("[$10] = $%02x; [$1A] = $%02x\n", data[0x10], data[0x1A])
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

func makeVGET(addr uint32, size uint8) []byte {
	sb := make([]byte, 64)
	sb[0] = byte('U')
	sb[1] = byte('S')
	sb[2] = byte('B')
	sb[3] = byte('A')
	sb[4] = byte(OpVGET)
	sb[5] = byte(SpaceSNES)
	sb[6] = byte(FlagDATA64B | FlagNORESP)
	// 4-byte struct: 1 byte size, 3 byte address
	sb[32] = byte(size)
	sb[33] = byte((addr >> 16) & 0xFF)
	sb[34] = byte((addr >> 8) & 0xFF)
	sb[35] = byte((addr >> 0) & 0xFF)
	return sb
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
