# ADR-003: Hardware - Pico W and HUB75 LED Matrix

## Status

Proposed

## Context

This ADR documents the hardware components and interfaces for the LED matrix driver project.

## Hardware Components

### 1. Raspberry Pi Pico W

The Pico W is an RP2040-based microcontroller with integrated WiFi/Bluetooth via the CYW43439 chip.

| Spec | Value |
|------|-------|
| MCU | RP2040 dual-core Cortex-M0+ @ 133MHz |
| RAM | 264KB SRAM |
| Flash | 2MB QSPI |
| WiFi | CYW43439 802.11n (2.4GHz) |
| GPIO | 26 multi-function pins |
| PIO | 2 × PIO blocks, 4 state machines each |

### 2. HUB75 LED Matrix Panel

HUB75 is a parallel data protocol used by RGB LED matrix panels. Common sizes: 32x16, 32x32, 64x32, 64x64.

| Spec | Typical Value |
|------|---------------|
| Pixel pitch | 2mm - 10mm |
| Scan rate | 1/8, 1/16, 1/32 |
| Color depth | 6-12 bit per channel |
| Voltage | 5V logic, 5V power |
| Refresh | 100-1000 Hz (software dependent) |

## HUB75 Protocol

### Pinout (16-pin IDC connector)

```
┌─────────────────────────────────┐
│  R0   G0   B0   GND   R1   G1  │  ← RGB data (2 rows)
│  B1   GND   A    B    C    D   │  ← Row address (A-D, E for 64 rows)
│  CLK  LAT  OE  GND             │  ← Control signals
└─────────────────────────────────┘
```

| Signal | Function |
|--------|----------|
| R0, G0, B0 | RGB data for upper half of panel |
| R1, G1, B1 | RGB data for lower half of panel |
| A, B, C, D, (E) | Row address select (binary encoded) |
| CLK | Shift register clock (rising edge shifts data) |
| LAT/STB | Latch - transfers shift register to output |
| OE | Output Enable (active low) |

### How It Works

1. **Two-row parallel update**: Panel updates 2 rows simultaneously
   - R0/G0/B0 controls row N (upper half)
   - R1/G1/B1 controls row N+16 (lower half, on 32-row panel)

2. **Shift register chain**: Each row is a shift register
   - Clock in 1 bit per pixel per color channel
   - 64-pixel wide panel = 64 clock cycles per row pair

3. **Row scanning**: Cycle through all row pairs
   - Set address lines (A, B, C, D) to select row pair
   - Shift in pixel data
   - Pulse LAT to latch data to outputs
   - Enable OE to display

4. **Binary Code Modulation (BCM)**: For color depth
   - Display each bit plane for time proportional to its weight
   - 8-bit color = 8 passes with 1:2:4:8:16:32:64:128 timing ratio

### Timing Diagram

```
CLK  ─┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──
      └──┘  └──┘  └──┘  └──┘  └──┘  └──┘

RGB  ══X═══X═══X═══X═══X═══X═══════════════
       P0   P1   P2   P3   P4   ...

LAT  ─────────────────────────────┐  ┌─────
                                  └──┘

OE   ─────────────────────────────────┐  ┌─
                                      └──┘
```

### Refresh Rate Calculation

```
refresh_rate = clock_freq / (pixels_per_row × row_pairs × bit_depth × bcm_overhead)

Example: 64-wide, 32-row, 8-bit color
- 64 pixels × 16 row pairs × 8 bits × 2 (BCM avg) = 16,384 clocks/frame
- At 10MHz clock: 10,000,000 / 16,384 ≈ 610 Hz refresh
```

## Pico W Pin Mapping

### HUB75 Connection (using PIO)

```
Pico Pin   │ HUB75 Signal │ Notes
───────────┼──────────────┼─────────────────
GPIO 0     │ R0           │ PIO controlled
GPIO 1     │ G0           │
GPIO 2     │ B0           │
GPIO 3     │ R1           │
GPIO 4     │ G1           │
GPIO 5     │ B1           │
GPIO 6     │ CLK          │ High-speed clock
GPIO 7     │ LAT          │ Directly set by PIO
GPIO 8     │ OE           │ PWM for brightness
GPIO 9     │ A            │ Row address
GPIO 10    │ B            │
GPIO 11    │ C            │
GPIO 12    │ D            │
GPIO 13    │ E            │ (64-row panels only)
```

### Reserved Pins

```
Pico Pin   │ Function     │ Notes
───────────┼──────────────┼─────────────────
GPIO 23    │ CYW43 WL_ON  │ WiFi chip enable
GPIO 24    │ CYW43 DATA   │ SPI data (shared)
GPIO 25    │ CYW43 CS     │ SPI chip select
GPIO 29    │ CYW43 CLK    │ SPI clock
```

## CYW43439 WiFi Chip

### TinyGo Driver

The [soypat/cyw43439](https://github.com/soypat/cyw43439) project provides a baremetal, heapless driver for the CYW43439 WiFi chip on the Pico W.

**Key features:**
- No heap allocations (buffers stored in-struct as arrays)
- Bare metal (no OS required)
- Compatible with TinyGo
- Designed to work with [soypat/lneto](https://github.com/soypat/lneto) TCP/IP stack

### Driver Architecture

```
┌─────────────────────────────────────────────┐
│              Application                     │
├─────────────────────────────────────────────┤
│              lneto (TCP/IP)                  │
├─────────────────────────────────────────────┤
│              cyw43439 driver                 │
├─────────────────────────────────────────────┤
│              PIO SPI (gSPI)                  │
├─────────────────────────────────────────────┤
│              CYW43439 Chip                   │
│         (runs firmware internally)           │
└─────────────────────────────────────────────┘
```

### WiFi Initialization Sequence

1. Power on CYW43439 (GPIO 23)
2. Load firmware blob to chip RAM
3. Initialize gSPI communication
4. Configure WiFi (SSID, password)
5. Associate with access point
6. DHCP for IP address (via lneto)

## PIO (Programmable I/O)

The RP2040's PIO is critical for jitter-free LED matrix driving.

### Why PIO?

| Approach | Jitter | CPU Load | Max Clock |
|----------|--------|----------|-----------|
| GPIO bitbang | High | 100% | ~1 MHz |
| DMA + timer | Medium | Low | ~10 MHz |
| **PIO** | **Zero** | **Zero** | **~60 MHz** |

### PIO for HUB75

Each PIO block has:
- 4 state machines
- 32 instructions shared
- Independent FIFOs (4×32-bit TX, 4×32-bit RX)
- DMA support

**State machine allocation:**
- SM0: RGB data shift (clocks out R0,G0,B0,R1,G1,B1)
- SM1: Row address control (sets A,B,C,D,E)
- DMA: Feeds pixel data to SM0 FIFO

### PIO Assembly (hub75.pio)

```pio
; HUB75 RGB shift - outputs 6 bits per clock
.program hub75_data
.side_set 1

.wrap_target
    out pins, 6     side 0  ; Output RGB data, CLK low
    nop             side 1  ; CLK high (shift into panel)
.wrap

; Row latch and address
.program hub75_row
    pull block              ; Wait for row data (address + control)
    out pins, 5             ; Set address lines (A,B,C,D,E)
    set pins, 1             ; LAT high
    set pins, 0             ; LAT low
```

### DMA Chain

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Framebuffer │────▶│ DMA Chan 0  │────▶│ PIO SM0 TX  │
│ (RAM)       │     │ (pixel data)│     │ FIFO        │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼ (triggers on completion)
                    ┌─────────────┐
                    │ DMA Chan 1  │────▶ Next row
                    │ (chain)     │
                    └─────────────┘
```

## Level Shifting

**Problem**: Pico GPIO is 3.3V, HUB75 expects 5V logic.

**Solutions:**

1. **74HCT245** - Octal bus transceiver (recommended)
   - 8 channels per chip, need 2 for HUB75
   - Fast switching (~10ns)
   - Unidirectional

2. **Direct connection** - Often works
   - HUB75 panels typically accept 3.3V as logic high
   - May be unreliable at higher clock speeds

3. **Pre-built boards**
   - [Pimoroni Interstate 75](https://shop.pimoroni.com/en-us/products/interstate-75-w) - Pico W with HUB75 driver built-in

## Power Requirements

### LED Panel

| Panel Size | Typical Current | Peak Current |
|------------|-----------------|--------------|
| 32x16 | 2A | 4A |
| 32x32 | 4A | 8A |
| 64x32 | 4A | 10A |
| 64x64 | 8A | 20A |

**Note**: Peak current assumes all LEDs white at full brightness. Actual usage much lower.

### Power Supply Sizing

```
Power (W) = rows × cols × 3 × 0.02A × duty_cycle × brightness

Example: 64x32 panel, 1/16 scan, 50% brightness
= 64 × 32 × 3 × 0.02 × (1/16) × 0.5
= 3.84W ≈ 0.8A @ 5V
```

Recommend: 5V 4A supply for 64x32 panel with headroom.

## Code Structure Updates

Based on hardware analysis, update [internal/led/](internal/led/):

```go
// internal/led/hub75.go
type HUB75Config struct {
    Rows       int    // Panel rows (16, 32, 64)
    Cols       int    // Panel columns (32, 64, 128)
    ScanRate   int    // 1/8, 1/16, 1/32
    ColorDepth int    // Bits per channel (1-8)

    // Pin assignments
    DataPins   [6]machine.Pin  // R0,G0,B0,R1,G1,B1
    AddrPins   [5]machine.Pin  // A,B,C,D,E
    ClkPin     machine.Pin
    LatPin     machine.Pin
    OEPin      machine.Pin
}

// internal/led/matrix_pico.go (//go:build pico)
type PicoMatrix struct {
    config     HUB75Config
    framebuf   []uint32        // Packed pixel data for DMA
    pioBlock   pio.PIO
    dataSM     pio.StateMachine
    rowSM      pio.StateMachine
    dmaChan    dma.Channel
}

func (m *PicoMatrix) Init(cfg HUB75Config) error {
    // 1. Configure GPIO pins
    // 2. Load PIO programs
    // 3. Configure DMA
    // 4. Start state machines
}

func (m *PicoMatrix) SetFrame(f *Frame) error {
    // Convert Frame to packed DMA format
    // Trigger DMA transfer
}
```

## References

- [soypat/cyw43439](https://github.com/soypat/cyw43439) - Baremetal CYW43439 driver for TinyGo
- [soypat/lneto](https://github.com/soypat/lneto) - Userspace TCP/IP stack
- [JuPfu/hub75](https://github.com/JuPfu/hub75) - HUB75 driver with PIO/DMA
- [pitschu/RP2040matrix-v2](https://github.com/pitschu/RP2040matrix-v2) - 64x64 matrix at 50Hz with FreeRTOS
- [HUB75 Protocol Guide](https://learn.lushaylabs.com/led-panel-hub75/) - Tang Nano 9K tutorial
- [Adafruit RGB Matrix Guide](https://cdn-learn.adafruit.com/downloads/pdf/32x16-32x32-rgb-led-matrix.pdf)
- [hzeller/rpi-rgb-led-matrix](https://github.com/hzeller/rpi-rgb-led-matrix/blob/master/wiring.md) - Wiring reference
- [Pimoroni Interstate 75 W](https://shop.pimoroni.com/en-us/products/interstate-75-w) - Pre-built HUB75 driver board
