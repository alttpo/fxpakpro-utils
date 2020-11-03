package main

import (
	"fmt"
	"log"
	"runtime"
	"time"
)

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

// This performs quite poorly in terms of scheduling jitter.
func testChannelTicker() {
	frameTicker := time.NewTicker(time.Nanosecond * time.Duration(snes_frame_time_nanoseconds_int))

	// Test the ticker scheduler:
	frames := 0
	last_t := time.Now()
mainloop:
	for {
		select {
		case t := <-frameTicker.C:
			frames++
			//log.Printf("delta %v ns; %d frames remaining\n", t.Sub(last_t).Nanoseconds(), 600 - frames)
			fmt.Printf("%v\n", t.Sub(last_t).Nanoseconds())
			last_t = t
			if frames >= 600 {
				break mainloop
			}
		}
	}

	frameTicker.Stop()
}

// Tests performed on my MacBook Pro 2018 2.6 GHz Intel Core i7

// Decent. This achieves +/- 47,840 ns [-9,612 .. +38,228] and all points are scattered about the range.
func testSleepLoop() {
	runtime.LockOSThread()

	frames := 0
	const dur = time.Nanosecond * time.Duration(snes_frame_time_nanoseconds_int)

	last_t := time.Now()

	const stepSize = 10_000

	for frames < 600 {
		frames++

		for time.Since(last_t) <= dur-(stepSize*2) {
			time.Sleep(time.Duration(stepSize) * time.Nanosecond)
		}
		t := time.Now()

		//log.Printf("delta %v ns; %d frames remaining\n", t.Sub(last_t).Nanoseconds(), 600 - frames)
		fmt.Printf("%v\n", t.Sub(last_t).Nanoseconds())
		last_t = t
	}

	runtime.UnlockOSThread()
}

// Excellent. This achieves +/- 49,882 ns [-152 .. +50,034] but all points are much closer to median.
func testSpinLoop() {
	runtime.LockOSThread()

	frames := 0
	const dur = time.Nanosecond * time.Duration(snes_frame_time_nanoseconds_int)

	last_t := time.Now()

	for frames < 600 {
		frames++

		for time.Since(last_t) < dur {
		}
		t := time.Now()

		//log.Printf("delta %v ns; %d frames remaining\n", t.Sub(last_t).Nanoseconds(), 600 - frames)
		fmt.Printf("%v\n", t.Sub(last_t).Nanoseconds())
		last_t = t
	}

	runtime.UnlockOSThread()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)
	log.Printf("SNES frame should take %v ns\n", snes_frame_time_nanoseconds_int)

	//testChannelTicker()
	//testSleepLoop()
	testSpinLoop()
}
