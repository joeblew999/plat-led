# ADR-004: PIO Library - tinygo-org/pio with RMII Support

## Status

Proposed

## Context

The [tinygo-org/pio](https://github.com/tinygo-org/pio) library provides a clean Go API for the RP2040/RP2350 Programmable I/O (PIO) block. The `rmii2` branch adds **RMII (Reduced Media Independent Interface)** support for 100Mbps wired Ethernet - a potential alternative to WiFi.

## What is tinygo-org/pio?

A TinyGo library that provides:
- Clean API for PIO state machine control
- DMA support for zero-CPU data transfer
- Pre-built drivers for common protocols

### Included Drivers (piolib package)

| Driver | Purpose |
|--------|---------|
| `SPI` | SPI master/slave |
| `Parallel` | 8-pin send-only parallel bus |
| `WS2812` | Neopixel LED driver |
| `Pulsar` | Pulse-constrained wave generator |
| `I2S` | Audio interface |
| **`RMIITxRx`** | 100Mbps Ethernet PHY (rmii2 branch) |

## RMII Support (rmii2 Branch)

### What is RMII?

RMII (Reduced Media Independent Interface) is a standard for connecting a microcontroller to an Ethernet PHY chip. It uses fewer pins than MII while providing 100Mbps Ethernet.

### Why RMII Matters for This Project

| Feature | WiFi (CYW43439) | RMII Ethernet |
|---------|-----------------|---------------|
| Speed | ~10-20 Mbps effective | 100 Mbps |
| Latency | Variable (10-100ms) | Deterministic (~1ms) |
| Reliability | Subject to interference | Wired, no interference |
| Power | Higher (radio) | Lower |
| Complexity | Firmware blob required | Pure hardware |

**For real-time LED control**, wired Ethernet via RMII could provide more deterministic timing than WiFi.

### RMII Pin Requirements

```
Pico Pin   │ RMII Signal  │ Direction │ Notes
───────────┼──────────────┼───────────┼─────────────────
GPIO x     │ TXD0         │ Output    │ 3 consecutive pins
GPIO x+1   │ TXD1         │ Output    │
GPIO x+2   │ TX_EN        │ Output    │
GPIO y     │ RXD0         │ Input     │ 2 consecutive pins
GPIO y+1   │ RXD1         │ Input     │
GPIO z     │ CRS_DV       │ Input     │ Carrier sense / data valid
GPIO w     │ REF_CLK      │ Input     │ 50MHz from PHY
```

### RMIITxRx API

```go
// From tinygo-org/pio rmii2 branch
type RMIITxRxConfig struct {
    Baud      uint32       // Clock divider
    TxPin     machine.Pin  // Base pin for TX (TXD0, TXD1, TX_EN)
    RxPin     machine.Pin  // Base pin for RX (RXD0, RXD1)
    CRSDVPin  machine.Pin  // Carrier Sense/Data Valid
    RefClkPin machine.Pin  // 50MHz reference clock
}

// Create RMII interface using 2 PIO state machines
rmii, err := piolib.NewRMIITxRx(smTx, smRx, cfg)

// Receive
rmii.SetRxHandler(buf, func(data []byte) {
    // Called when frame received (CRS_DV falls)
})
rmii.StartRx()

// Transmit
rmii.Tx8(frameData)
```

### How It Works

1. **TX State Machine**: Outputs 3 bits per clock (TXD0, TXD1, TX_EN)
2. **RX State Machine**: Waits for CRS_DV, then shifts in 2 bits per clock
3. **DMA**: Both TX and RX use DMA for zero-CPU transfer
4. **GPIO Interrupt**: CRS_DV falling edge triggers RX completion callback

### PIO Programs (Generated)

```pio
; TX: Output TXD0, TXD1, TX_EN (3 pins)
.program rmii_tx
    out pins, 3         ; Output 3 bits from TX FIFO

; RX: Wait for sync, read RXD0, RXD1 (2 pins)
.program rmii_rx
    wait 0 pin 2        ; Wait for CRS_DV low
    wait 0 pin 0        ; Sync on RXD0
    wait 0 pin 1        ; Sync on RXD1
    wait 1 pin 2        ; Wait for CRS_DV high (frame start)
    wait 1 pin 0        ; Sync
    wait 1 pin 1        ; Sync
.wrap_target
    in pins, 2          ; Shift in 2 bits from RXD
.wrap
```

## Integration with lneto

The RMII driver provides raw Ethernet frame TX/RX. Combined with [soypat/lneto](https://github.com/soypat/lneto):

```
┌──────────────────────────────────────┐
│            Application               │
├──────────────────────────────────────┤
│         lneto (TCP/IP stack)         │
├──────────────────────────────────────┤
│    RMII NetIF adapter (new code)     │
├──────────────────────────────────────┤
│   piolib.RMIITxRx (rmii2 branch)     │
├──────────────────────────────────────┤
│      PIO + DMA (hardware)            │
├──────────────────────────────────────┤
│       Ethernet PHY chip              │
│     (e.g., LAN8720, DP83848)         │
└──────────────────────────────────────┘
```

## Hardware Options

### Ethernet PHY Chips Compatible with RMII

| Chip | Speed | Notes |
|------|-------|-------|
| LAN8720A | 10/100 Mbps | Common, cheap, 3.3V |
| DP83848 | 10/100 Mbps | Texas Instruments |
| KSZ8081 | 10/100 Mbps | Microchip |

### Pre-built Boards

| Board | PHY | Notes |
|-------|-----|-------|
| W5500-EVB-Pico | W5500 (SPI) | Not RMII, but alternative |
| Custom | LAN8720A | Wire to Pico GPIO |

## Decision

**Adopt tinygo-org/pio `rmii2` branch** for:
1. HUB75 LED matrix driving (PIO + DMA)
2. Optional wired Ethernet via RMII for deterministic networking

### Networking Strategy

| Scenario | Interface | Rationale |
|----------|-----------|-----------|
| Default | WiFi (CYW43439) | Built into Pico W, no extra hardware |
| Low-latency | RMII Ethernet | Deterministic timing for real-time sync |
| Desktop sim | TAP interface | Development testing |

## Code Structure Updates

```
├── internal/
│   ├── netif/
│   │   ├── netif.go               # Interface definition
│   │   ├── netif_wifi_pico.go     # //go:build pico && wifi - CYW43439
│   │   ├── netif_rmii_pico.go     # //go:build pico && rmii - RMII via PIO
│   │   ├── netif_tap_darwin.go    # //go:build darwin
│   │   ├── netif_tap_windows.go   # //go:build windows
│   │   └── netif_tap_linux.go     # //go:build linux
```

### Build Tags for Network Interface

```bash
# WiFi (default for Pico W)
tinygo build -tags=wifi -target=pico -o led-driver.uf2 ./cmd/led-driver

# RMII Ethernet (requires external PHY)
tinygo build -tags=rmii -target=pico -o led-driver.uf2 ./cmd/led-driver
```

## DMA Helper Code

The `rmii2` branch also adds robust DMA support in `piolib/dma.go`:

```go
// DMA arbiter for claiming channels
_DMA.ClaimChannel() (dmaChannel, bool)

// Push data to peripheral (e.g., PIO TX FIFO)
ch.Push8(dst, src, dreq)
ch.Push16(dst, src, dreq)
ch.Push32(dst, src, dreq)

// Pull data from peripheral (e.g., PIO RX FIFO)
ch.Pull8(dst, src, dreq)
ch.Pull16(dst, src, dreq)
ch.Pull32(dst, src, dreq)
```

This DMA code is also useful for the HUB75 LED driver.

## Consequences

- Dependency on tinygo-org/pio `rmii2` branch (not yet merged to main)
- Optional Ethernet PHY hardware for RMII mode
- Build tags differentiate WiFi vs RMII builds
- DMA helpers simplify both LED and network drivers

## References

- https://github.com/tinygo-org/pio (main)
- https://github.com/tinygo-org/pio/tree/rmii2 (RMII branch)
- https://github.com/sandeepmistry/pico-rmii-ethernet (inspiration for RMII driver)
- https://github.com/soypat/lneto (TCP/IP stack)
- [RP2040 Datasheet - PIO](https://datasheets.raspberrypi.com/rp2040/rp2040-datasheet.pdf#page=310)
- [LAN8720A Datasheet](https://www.microchip.com/en-us/product/lan8720a)
