package main

import (
	"fmt"
	"github.com/aybabtme/uniplot/histogram"
	"golang.org/x/text/message"
	"log"
	"math"
	"time"
)

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

func cleanData(a []float64) (cleaned []float64, outliers []float64) {
	cleaned = make([]float64, 0, len(a))
	for i := 1; i < len(a); i += 2 {
		if math.Abs(a[i]-a[i-1]) >= 16_667_000 {
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
