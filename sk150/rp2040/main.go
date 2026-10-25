//go:build tinygo

// Command rp2040 is the SK150 bridge firmware for a Raspberry Pi Pico: the
// board's USB port is a serial port to the host, and UART0 owns the SK150's
// serial header. See README.md for the wiring and the flashing steps.
package main

import (
	"machine"
	"time"
)

// Pin assignment, all on the Pico's right-hand edge (see README.md).
const (
	devTX = machine.GP16 // physical pin 21, to the SK150's RX
	devRX = machine.GP17 // physical pin 22, from the SK150's TX
	dbgTX = machine.GP20 // physical pin 26, diagnostics out at 115200
	dbgRX = machine.GP21 // physical pin 27
)

func main() {
	machine.LED.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Diagnostics go to UART1, never to the USB port: that carries Modbus.
	dbg := machine.UART1
	dbg.Configure(machine.UARTConfig{BaudRate: 115200, TX: dbgTX, RX: dbgRX})
	logTo = dbg
	logs("sk150 bridge starting")

	dev := machine.UART0
	dev.Configure(machine.UARTConfig{BaudRate: defaultBaud, TX: devTX, RX: devRX})
	l := newLink(dev, dev.SetBaudRate)
	l.busy = machine.LED.Set

	// The SK150 powers this board, so it is usually still booting its own
	// UART when we get here; give it a moment before the first probe.
	time.Sleep(300 * time.Millisecond)
	l.Probe()

	p := newProxy(machine.Serial, l)
	for {
		if !p.step() {
			time.Sleep(pollSleep)
		}
	}
}
