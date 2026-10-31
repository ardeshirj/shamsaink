module einkengine

go 1.27

require (
	einkcre v0.0.0
	einklayout v0.0.0
	github.com/readium/go-toolkit v0.16.1
	shamsainkengine v0.0.0
)

require (
	github.com/agext/regexp v1.3.0 // indirect
	github.com/andybalholm/cascadia v1.3.5 // indirect
	github.com/antchfx/xmlquery v1.5.1 // indirect
	github.com/antchfx/xpath v1.3.8 // indirect
	github.com/deckarep/golang-set v1.8.0 // indirect
	github.com/go-text/typesetting v0.3.6-0.20261001104650-7f66441ebcc8 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/readium/zran v0.0.0-20260525212206-dcf56adb2c0e // indirect
	github.com/relvacode/iso8601 v1.8.0 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// the gotext and cre engines are experiments' own packages, shamsainkengine is the Blitz engine's binding (engine/go);
// go-toolkit is experiment 02's trimmed copy (trim-gotoolkit.sh)
replace (
	einkcre => ../experiments/02-crengine-go
	einklayout => ../experiments/03-own-layout
	shamsainkengine => ../engine/go
	github.com/readium/go-toolkit => ../experiments/02-crengine-go/patched/go-toolkit
)
