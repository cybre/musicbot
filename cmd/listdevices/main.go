// Package main provides a utility to list available PortAudio devices.
package main

import (
	"fmt"
	"log"

	"github.com/gordonklaus/portaudio"
)

func main() {
	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize PortAudio: %v", err)
	}
	defer func() {
		if err := portaudio.Terminate(); err != nil {
			log.Printf("Failed to terminate PortAudio: %v", err)
		}
	}()

	devices, err := portaudio.Devices()
	if err != nil {
		log.Fatalf("Failed to list devices: %v", err)
	}

	fmt.Println("Available PortAudio Devices:")
	fmt.Println("============================")
	fmt.Println()

	for i, device := range devices {
		fmt.Printf("[%d] %s\n", i, device.Name)
		fmt.Printf("    Max Input Channels:  %d\n", device.MaxInputChannels)
		fmt.Printf("    Max Output Channels: %d\n", device.MaxOutputChannels)
		fmt.Printf("    Default Sample Rate: %.0f Hz\n", device.DefaultSampleRate)

		if device.MaxInputChannels > 0 {
			fmt.Printf("    Default Low Input Latency:  %v\n", device.DefaultLowInputLatency)
			fmt.Printf("    Default High Input Latency: %v\n", device.DefaultHighInputLatency)
		}

		if device.MaxOutputChannels > 0 {
			fmt.Printf("    Default Low Output Latency:  %v\n", device.DefaultLowOutputLatency)
			fmt.Printf("    Default High Output Latency: %v\n", device.DefaultHighOutputLatency)
		}

		fmt.Println()
	}

	defaultInput, err := portaudio.DefaultInputDevice()
	if err == nil {
		fmt.Printf("Default Input Device: %s\n", defaultInput.Name)
	}

	defaultOutput, err := portaudio.DefaultOutputDevice()
	if err == nil {
		fmt.Printf("Default Output Device: %s\n", defaultOutput.Name)
	}
}
