//go:build !linux

package main

func prepareDisplay() (func(), error) {
	return func() {}, nil
}
