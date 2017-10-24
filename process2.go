package main

import (
	"github.com/ledyba/go-fft/fft"
	"github.com/youpy/go-wav"
	"io"
	"log"
	"math"
	"math/cmplx"
	"os"
)

const winsize = 128

func Sft(data []complex128) {
	for i := range data {
		data[i] = complex(imag(data[i]), real(data[i]))
	}

	if len(data) == 1 {
		return
	}

	result := make([]complex128, len(data))

	for j := 0; j < len(data); j++ {
		outIndex := float64(j)
		outHz := indexToHz(j)

		//INSERT TRANSFORMATION HERE
		modifiedOutHz := invertFreq(outHz)

		if !audible(outHz) || !audible(modifiedOutHz) {
			continue
		}

		var high = j > winsize/2

		newIndex := hzToIndexFrac(modifiedOutHz, high)
		outIndex = newIndex

		for i := 0; i < len(data); i++ {
			result[i] += data[j] * cmplx.Exp(complex(0, 2*math.Pi*(float64(i)*outIndex)/float64(len(data))))
		}
	}

	for i := 0; i < len(data); i++ {
		data[i] = result[i]
	}
	scale := 1.0 / float64(len(data))
	for i := range data {
		data[i] = complex(imag(data[i])*scale, real(data[i])*scale)
	}
}

var (
	sampleRate int
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

func inside(x int, a int, b int) bool {
	return x > a && x < b
}

func audible(f float64) bool {
	return f >= 20 && f <= 20000
}

func indexToHz(i int) float64 {
	if i > winsize/2 {
		i = winsize - i
	}
	return float64(i*44100) / float64(winsize)
}

func hzToIndex(hz float64, high bool) int {
	result := int(hz * float64(winsize) / 44100)
	if high && result != 0 {
		result = winsize - result
	}
	return result
}

func hzToIndexFrac(hz float64, high bool) float64 {
	result := hz * float64(winsize) / 44100
	if high && result >= 1 {
		result = winsize - result
	}
	return result
}

func removeInaudible(data []complex128) {
	for i := range data {
		f := indexToHz(i)
		if !audible(f) {
			data[i] = 0
		}
	}
}

func mirror(data []complex128) {
	l := len(data)
	//i:=1 is deliberate, because data[0] is not mirrored
	for i := 1; i < l/2-1; i++ {
		data[l-i] = data[i]
	}
}

func invertFreq(f float64) float64 {
	return 1024/(f/1024) - 300
}

func scaleFreq(f float64) float64 {
	return 260 * math.Pow(f/260, 0.5)
}

func translateFreq(f float64) float64 {
	return f - 100
}

func win(samples []complex128) []complex128 {
	samples = samples[0 : (len(samples)/winsize)*winsize]
	out := make([]complex128, len(samples))
	donec := make(chan struct{})
	nworkers := 4
	for chunk := 0; chunk < nworkers; chunk++ {
		chunk := chunk
		go func() {
			for i := chunk * (len(samples) / nworkers); i+winsize < (chunk+1)*len(samples)/nworkers; i += winsize / 2 {
				x := make([]complex128, winsize)
				copy(x, samples[i:i+winsize])
				fft.Fft(x)
				removeInaudible(x)
				//mirror(x)
				Sft(x)
				for j := 0; j < winsize/2; j++ {
					out[i+j] += x[j] * c(float64(j)/(float64(winsize)/2))
				}
				for k := winsize / 2; k < winsize; k++ {
					out[i+k] += x[k] * c(float64(winsize-k)/(float64(winsize)/2))
				}
			}
			donec <- struct{}{}
		}()
	}
	for i := 0; i < nworkers; i++ {
		<-donec
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

	sampleRate = int(hdr.SampleRate)

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
