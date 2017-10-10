package main

import (
	"io"
	"log"
	"os"

	"github.com/ledyba/go-fft/fft"
	"github.com/youpy/go-wav"
)

func crunch(samples []int) {
	s := 8
	for i := 0; i < len(samples); i += s {
		samples[i] = (samples[i] << 8) >> 8
		for j := 1; j < s; j++ {
			samples[j+i] = samples[i]
		}
	}
}

func manip(f complex128) complex128 {
	return f * 2
}

func c(f float64) complex128 {
	return complex(f, 0)
}

func win(samples []complex128) []complex128 {
	winsize := 512
	samples = samples[0 : (len(samples)/winsize)*winsize]
	out := make([]complex128, len(samples))
	for i := 0; i+winsize < len(samples); i += winsize / 2 {
		x := make([]complex128, winsize)
		copy(x, samples[i:i+winsize])
		fft.Fft(x)

		//		for j := range x {
		//			x[j] /= 2
		//		}

		//skip := winsize / 4
		// for j := 0; j < len(x)-skip; j++ {
		// 	x[j+skip] = x[j]
		// }

		h := len(x) / 2
		middleDrop := int((0.95) * float64(h))
		for j := range x {
			if !((j < h-middleDrop) || (j > h+middleDrop)) {
				x[j] = 0
			}
		}

		fft.InvFft(x)
		for j := 0; j < winsize/2; j++ {
			out[i+j] += x[j] * c(float64(j)/(float64(winsize)/2))
		}
		for k := winsize / 2; k < winsize; k++ {
			out[i+k] += x[k] * c(float64(winsize-k)/(float64(winsize)/2))
		}
	}
	return out
}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: simpleRead <file.wav>\n")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("couldn't open file: %v", err)
	}

	r := wav.NewReader(f)

	hdr, err := r.Format()
	if err != nil {
		log.Fatalf("error reading header %v", err)
	}

	channels := [2][]complex128{}

	for {
		ss, err := r.ReadSamples()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("error reading sample %v", err)
		}

		for i := range ss {
			channels[0] = append(channels[0], complex(float64(ss[i].Values[0])/float64(int(1)<<hdr.BitsPerSample), 0))
			channels[1] = append(channels[1], complex(float64(ss[i].Values[1])/float64(int(1)<<hdr.BitsPerSample), 0))
		}
	}

	channels[0] = win(channels[0])
	channels[1] = win(channels[1])

	outf, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalf("error saving file %v", err)
	}

	w := wav.NewWriter(outf, uint32(len(channels[0])), hdr.NumChannels, hdr.SampleRate, hdr.BitsPerSample)

	osamples := make([]wav.Sample, 1)
	for i := range channels[0] {
		osamples[0].Values[0] = int(real(channels[0][i]) * float64(int(1)<<hdr.BitsPerSample))
		osamples[0].Values[1] = int(real(channels[1][i]) * float64(int(1)<<hdr.BitsPerSample))

		err = w.WriteSamples(osamples)
		if err != nil {
			log.Fatalf("error writing sample %v", err)
		}
	}

	outf.Close()
}
