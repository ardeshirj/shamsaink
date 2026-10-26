module indiumreader

go 1.27

require (
	github.com/readium/go-toolkit v0.16.1
	indiumengine v0.0.0
)

require (
	github.com/agext/regexp v1.3.0 // indirect
	github.com/andybalholm/cascadia v1.3.5 // indirect
	github.com/antchfx/xmlquery v1.5.1 // indirect
	github.com/antchfx/xpath v1.3.8 // indirect
	github.com/deckarep/golang-set v1.8.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/readium/zran v0.0.0-20260525212206-dcf56adb2c0e // indirect
	github.com/relvacode/iso8601 v1.8.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

// the engine's Go binding; go-toolkit is experiment 02's trimmed copy (trim-gotoolkit.sh)
replace (
	github.com/readium/go-toolkit => ../experiments/02-crengine-go/patched/go-toolkit
	indiumengine => ../engine/go
)
