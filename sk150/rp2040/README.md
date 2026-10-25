# SK150 bridge firmware for the Raspberry Pi Pico

A TinyGo program that puts an RP2040 between the desktop app and the
XY-SK150. The Pico's USB port is a plain serial port to the computer; its
UART0 owns the SK150's serial header. Every Modbus RTU frame from the host
is forwarded to the device unchanged and the reply is relayed unchanged, so
the `sk150` app works exactly as it does over the FTDI cable, and the Pico
is left holding the one seat on the bus that later automation (input
cutoff from the battery voltage, per-input sensing) needs.

The one register the firmware interprets is the baud selector (0x19): the
SK150 adopts a new speed only at its next power cycle, so the bridge
remembers what it was told and, once the device stops answering, re-probes
with that speed first. At start-up it probes the whole table, so a device
switched from its own menu is found as well.

## Wiring

All on the Pico's right-hand edge. SK150 TX and RX are 3.3 V, so no level
shifting. The 5 V pin powers the Pico through a Schottky diode into VSYS;
the diode keeps the computer's USB 5 V out of the SK150.

| Pico pin | GPIO | Role | SK150 header |
|---|---|---|---|
| 21 | GP16 | UART0 TX | RX |
| 22 | GP17 | UART0 RX | TX |
| 23 | GND | 0 V return | GND |
| 39 | VSYS | power in | 5 V, via a 1N5819 (band toward VSYS) |

Optional diagnostics at 115200 8N1 on UART1: GP20 (pin 26) is TX, GP21
(pin 27) RX. Nothing is printed on the USB port, which carries Modbus.

The onboard LED lights while a request is out on the SK150 bus.

## Build and flash

    cd sk150/rp2040
    go test ./...                       # the framing and proxy logic, on the host
    tinygo build -target=pico -o sk150-bridge.uf2 .

This is its own module (the desktop app's Gio dependencies have no place
in a firmware image). In a checkout governed by the org's generated
`go.work`, which does not list it, prefix both commands with `GOWORK=off`.

Hold BOOTSEL while plugging the Pico in (or short TP6 to GND while
applying power), then either copy the `.uf2` onto the RPI-RP2 drive that
appears, or flash directly:

    tinygo flash -target=pico .

The Pico shows up on macOS as `/dev/cu.usbmodem*`, which the desktop app
already picks when no FTDI adapter is present; unplug the FTDI cable.

## Layout

- `modbus.go` — RTU framing: CRC, frame lengths, the host-side framer.
- `proxy.go` — the link (sole owner of the SK150 UART, with the baud
  probe) and the proxy loop. Hardware-free, tested with `go test`.
- `main.go` — the TinyGo entry point: pins, UARTs, USB, LED.
