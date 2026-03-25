# plat-led

led matrix hardware driver for pico real time.

2 optiosn to leverage for real time with no jiter and no os.

https://github.com/TinyCC/tinycc 

https://github.com/tinygo-org/pio

## hardware

led matrix

## networking

in adr 002 to get data in and out. but maybe also to help with provisioning ? 

curl too ? 
https://github.com/soypat/lneto/blob/main/examples/xcurl/main.go

## pio

https://github.com/tinygo-org/pio/tree/rmii2

The rmii2 branch adds RMII (Reduced Media Independent Interface) support for 100Mbps Ethernet over PIO. This could be an alternative to WiFi for wired Ethernet connectivity. Let me check for more context.

https://github.com/raspberrypi/pico-sdk

The pio repo already has pre-generated *_pio.go files. You only need pico-sdk if you modify .pio files and need to regenerate.

If you later need pioasm, it's just one tool from the SDK:


brew install pioasm   # Or build from pico-sdk/tools/pioasm
For now, you're good without it.


