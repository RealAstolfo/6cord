// +build !unix,!windows

package main

import "os"

func syscallSilenceStderr(f *os.File) {}