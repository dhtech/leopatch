// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !arm

package ast

import (
	"os"
)

type Ast struct {
	g   *amiGpio
	p   *os.File
	off int64
}

func NewAst() *Ast {
	p, err := os.OpenFile("/dev/port", os.O_RDWR, 0600)
	if err != nil {
		panic(err)
	}

	port := 0x2e

	l := &Ast{p: p, off: int64(port)}
	l.unlock()
	l.selectDevice(0xd)
	l.enable()

	return l
}

func (l *Ast) ctrl(d byte) {
	b := []byte{d}
	l.p.WriteAt(b, l.off)
}

func (l *Ast) wf(f int, d byte) {
	// Write F0-8 reg through cache to avoid redundant writes
	l.ctrl(byte(f))
	l.w(d)
}

func (l *Ast) w(d byte) {
	b := []byte{d}
	l.p.WriteAt(b, l.off+1)
}

func (l *Ast) r() byte {
	b := make([]byte, 1)
	l.p.ReadAt(b, l.off+1)
	return b[0]
}

func (l *Ast) enable() {
	// Enable SIO iLPC2AHB
	// TODO(bluecmd): Does this make sense? If it's not enabled we couldn't
	// enable it, right?
	l.ctrl(0x30)
	l.w(0x1)
}

func (l *Ast) unlock() {
	// Unlock SIO
	l.ctrl(0xa5)
	l.ctrl(0xa5)
}

func (l *Ast) Close() {
	// Lock SIO
	l.ctrl(0xaa)
}

func (l *Ast) selectDevice(d int) {
	l.ctrl(0x07)
	l.w(byte(d))
}

func (l *Ast) addr(a uintptr) {
	l.wf(0xf0, byte(a>>24&0xff))
	l.wf(0xf1, byte(a>>16&0xff))
	l.wf(0xf2, byte(a>>8&0xff))
	l.wf(0xf3, byte(a&0xff))
}

func (l *Ast) MustRead32(a uintptr) uint32 {
	l.addr(a)
	// Select 32 bit
	l.wf(0xf8, 0x2)
	// Trigger
	l.ctrl(0xfe)
	l.r()
	// Read 32 bit
	var res uint32
	l.ctrl(0xf4)
	f := l.r()
	res |= uint32(f) << 24
	l.ctrl(0xf5)
	f = l.r()
	res |= uint32(f) << 16
	l.ctrl(0xf6)
	f = l.r()
	res |= uint32(f) << 8
	l.ctrl(0xf7)
	f = l.r()
	res |= uint32(f)
	return res
}

func (l *Ast) MustRead8(a uintptr) uint8 {
	l.addr(a)
	// Select 8 bit
	l.wf(0xf8, 0)
	// Trigger
	l.ctrl(0xfe)
	l.r()
	// Read 8 bit
	// TODO(bluecmd) WHat about the other regs here?
	l.ctrl(0xf7)
	f := l.r()
	return f
}

func (l *Ast) MustWrite32(a uintptr, d uint32) {
	l.addr(a)
	// Select 32 bit
	l.wf(0xf8, 0x2)

	// Write 32 bit
	l.wf(0xf4, byte(d>>24&0xff))
	l.wf(0xf5, byte(d>>16&0xff))
	l.wf(0xf6, byte(d>>8&0xff))
	l.wf(0xf7, byte(d&0xff))
	// Trigger
	l.ctrl(0xfe)
	l.w(0xcf)
}

func (l *Ast) MustWrite8(a uintptr, d uint8) {
	l.addr(a)
	// Select 8 bit
	l.wf(0xf8, 0)
	// Write 8 bit
	l.wf(0xf7, byte(d&0xff))
	// Trigger
	l.ctrl(0xfe)
	l.w(0xcf)
}
