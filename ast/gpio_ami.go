// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/binary"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	REQUEST_GET_DIR = 1
	REQUEST_SET_DIR = 2
	REQUEST_GET_DATA = 7
	REQUEST_SET_DATA = 8

	PIN_POWER_SWITCH     = 27  // Active-low
	PIN_POWER_STATUS     = 34  // Active-high
	PIN_BIOS_ROM_SELECT  = 108
	PIN_BIOS_BMC_MASTER  = 109 // Active-high
)

var p *amiGpio

type amiGpio struct {
	g *os.File
}

func (a *amiGpio) mustIoctl(req int, arg []byte) {
	argp := uintptr(unsafe.Pointer(&arg[0]))
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, a.g.Fd(), uintptr(req), argp)
	if e != 0 {
		panic(os.NewSyscallError("ioctl", e))
	}
}

func (a *amiGpio) MustReadPin(pin int) bool {
	data := make([]byte, 3)
	binary.LittleEndian.PutUint16(data[0:], uint16(pin))
	a.mustIoctl(REQUEST_GET_DATA, data)
	return data[2] == 1
}

func (a *amiGpio) MustSetPin(pin int, high bool) {
	data := make([]byte, 3)
	binary.LittleEndian.PutUint16(data[0:], uint16(pin))
	if high {
		data[2] = 1
	} else {
		data[2] = 0
	}
	a.mustIoctl(REQUEST_SET_DATA, data)
}

func (a *amiGpio) MustSetPinDirection(pin int, out bool) {
	data := make([]byte, 3)
	binary.LittleEndian.PutUint16(data[0:], uint16(pin))
	if out {
		data[2] = 1
	} else {
		data[2] = 0
	}
	a.mustIoctl(REQUEST_SET_DIR, data)
}

func (a *Ast) IsPoweredOn() bool {
	// Power status is active high
	return a.g.MustReadPin(PIN_POWER_STATUS)
}

func (a *Ast) HoldPowerButton(dur time.Duration) {
	// Power switch is active low
	a.g.MustSetPin(PIN_POWER_SWITCH, false)
	time.Sleep(dur)
	a.g.MustSetPin(PIN_POWER_SWITCH, true)
}

func (a *Ast) SetBiosBmcMaster(master bool) {
	if master {
		a.g.MustSetPin(PIN_BIOS_BMC_MASTER, true)
		a.g.MustSetPinDirection(PIN_BIOS_BMC_MASTER, true)
	} else {
		a.g.MustSetPinDirection(PIN_BIOS_BMC_MASTER, false)
	}
}

func newAmiGpio() *amiGpio {
	g, err := os.OpenFile("/dev/gpio0", os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}
	return &amiGpio{g}
}
