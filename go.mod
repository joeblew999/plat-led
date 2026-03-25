module github.com/joeblew999/plat-led

go 1.25

// Local development: use cloned repos from .src/
// Run `task src:clone-all` to populate these directories
replace (
	github.com/soypat/cyw43439 => ./.src/cyw43439
	github.com/soypat/lneto => ./.src/lneto
	github.com/tinygo-org/pio => ./.src/pio
)
