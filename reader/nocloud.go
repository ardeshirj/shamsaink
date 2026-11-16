//go:build !readium_nocloud

package main

// go-toolkit without its build tag readium_nocloud links the AWS and Google Cloud SDKs: +27 MB of binary and +26 MB of
// memory at start-up (experiment 02). Every build of the reader takes the tag (-tags blitz,readium_nocloud), and this
// stops one that forgets it.
var _ = buildWithTag_readium_nocloud
