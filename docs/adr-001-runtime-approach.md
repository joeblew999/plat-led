# ADR-001: Runtime Approach for LED Matrix Driver

## Status

Proposed

## Context

We need a real-time LED matrix hardware driver for Raspberry Pi Pico that operates without jitter and without an operating system. The driver must have deterministic timing for proper LED matrix control.

Two approaches have been identified:

1. **TinyCC** - A small, fast C compiler that can target bare metal
2. **TinyGo with PIO** - Go compiler for microcontrollers with Programmable I/O support

### Requirements

- Real-time performance with no jitter
- No OS overhead
- LED matrix hardware control
- Cross-platform development tooling

## Decision

**TinyGo with PIO** - Go provides memory safety and better developer experience, while PIO handles precise timing requirements in hardware.

## Options

### Option 1: TinyCC

**Pros:**
- Minimal runtime overhead
- Direct hardware access
- Fast compilation
- Small binary size

**Cons:**
- Manual memory management
- Less type safety
- More boilerplate code

### Option 2: TinyGo with PIO

**Pros:**
- Memory safety with Go
- PIO support for precise timing offloaded to hardware
- Better developer experience
- Active community and Pico support

**Cons:**
- Slightly larger runtime
- Less control over low-level details

## Consequences

- TinyGo compiler required for Pico builds
- PIO programs handle timing-critical LED refresh
- Standard Go toolchain works for desktop simulation
- Cross-platform code sharing via build tags

## References

- https://github.com/TinyCC/tinycc
- https://github.com/tinygo-org/pio
