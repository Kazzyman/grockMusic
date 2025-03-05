package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"

	"github.com/gordonklaus/portaudio"
	"github.com/mjibson/go-dsp/fft"
)

const (
	sampleRate      = 44100 // CD-quality sample rate
	framesPerBuffer = 2048  // Increased for better resolution
)

type Note struct {
	Name      string
	Frequency float64
}

var pianoNotes = generatePianoNotes()

func generatePianoNotes() []Note {
	notes := make([]Note, 61)
	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	A4 := 440.0
	startFreq := A4 * math.Pow(2, -33.0/12.0)

	for i := 0; i < 61; i++ {
		freq := startFreq * math.Pow(2, float64(i)/12.0)
		octave := 2 + (i / 12)
		noteIdx := i % 12
		name := fmt.Sprintf("%s%d", noteNames[noteIdx], octave)
		notes[i] = Note{Name: name, Frequency: freq}
	}
	return notes
}

func main() {
	err := portaudio.Initialize()
	if err != nil {
		fmt.Printf("Error initializing PortAudio: %v\n", err)
		return
	}
	defer portaudio.Terminate()

	stream, err := portaudio.OpenDefaultStream(
		1,               // input channels (mono)
		0,               // output channels
		sampleRate,      // sample rate
		framesPerBuffer, // frames per buffer
		processAudio,    // callback function
	)
	if err != nil {
		fmt.Printf("Error opening stream: %v\n", err)
		return
	}
	defer stream.Close()

	err = stream.Start()
	if err != nil {
		fmt.Printf("Error starting stream: %v\n", err)
		return
	}

	fmt.Println("Listening for piano notes... Press Ctrl+C to stop.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	err = stream.Stop()
	if err != nil {
		fmt.Printf("Error stopping stream: %v\n", err)
	}
	fmt.Println("\nStopped listening.")
}

func processAudio(in []float32) {
	samples := make([]complex128, framesPerBuffer)
	for i, sample := range in {
		samples[i] = complex(float64(sample), 0)
	}

	freqDomain := fft.FFT(samples)
	magnitudes := make([]float64, framesPerBuffer/2)
	for i := 0; i < framesPerBuffer/2; i++ {
		magnitudes[i] = math.Sqrt(real(freqDomain[i])*real(freqDomain[i]) + imag(freqDomain[i])*imag(freqDomain[i]))
	}

	maxMag := 0.0
	maxIdx := 0
	freqResolution := float64(sampleRate) / float64(framesPerBuffer) // ~21.53 Hz
	highFreqLimit := int(2200.0 / freqResolution)                    // ~102 bins, up to ~2200 Hz
	for i := 0; i < highFreqLimit && i < len(magnitudes); i++ {
		if magnitudes[i] > maxMag {
			maxMag = magnitudes[i]
			maxIdx = i
		}
	}

	freq := float64(maxIdx) * freqResolution
	closestNote := findClosestNote(freq)

	// Harmonic correction
	if maxMag > 0.05 {
		adjustedFreq := freq
		if freq > 261.0 { // Above C4, check for harmonics
			fundamental := freq / 2.0
			if math.Abs(fundamental-findClosestNote(fundamental).Frequency) < freqResolution {
				adjustedFreq = fundamental
				closestNote = findClosestNote(adjustedFreq)
			}
		}
		fmt.Printf("\rDetected: %s (%.2f Hz) | Raw Freq: %.2f Hz | Magnitude: %.4f    ",
			closestNote.Name, closestNote.Frequency, freq, maxMag)
	}
}

func findClosestNote(freq float64) Note {
	closest := pianoNotes[0]
	minDiff := math.Abs(freq - closest.Frequency)
	for _, note := range pianoNotes[1:] {
		diff := math.Abs(freq - note.Frequency)
		if diff < minDiff {
			minDiff = diff
			closest = note
		}
	}
	return closest
}
