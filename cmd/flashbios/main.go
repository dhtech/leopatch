package main

import (
	"fmt"
	"os"
	"time"

	"github.com/dhtech/leopatch/ast"
	"github.com/dhtech/leopatch/mtd"
)

func flashBios(f *os.File, a *ast.Ast) {
	a.SetBiosBmcMaster(true)
	defer a.SetBiosBmcMaster(false)

	if !a.IsSpiMaster() {
		fmt.Println("Not SPI master, switching to master mode")
		a.SetSpiMaster(true)
	}
	defer a.SetSpiMaster(false)

	// Wait for MTD.
	// TODO(bluecmd): I don't know how to check for this in a good way.
	// It seems like Linux 2.6.28 which is what AMI BMC uses caches the MTD
	// device information.
	time.Sleep(1 * time.Second)

	// TODO(bluecmd): mtd path will probably not be the same in OpenBMC
	m, err := mtd.Open("/dev/mtd5")
	fmt.Printf("> Chip has size %v\n", m.Size)
	fmt.Printf("> Using erase size %v, write size %v\n", m.EraseSize, m.WriteSize)

	// Check file size for sanity
	size, err := f.Seek(0, 2)
	fmt.Printf("> File has size %v\n", size)
	if err != nil {
		panic(err)
	}
	if size > m.Size {
		panic("File size will not fit on ROM, aborting")
	}
	// TODO(bluecmd): Support partial flashing?
	if size != m.Size {
		panic("File is not for full ROM, partial flashing not supported")
	}

	fmt.Println("Erasing")
	m.Erase()
	fmt.Println("Writing")
	_, err = f.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	m.Write(f)

	fmt.Println("Verifying")
	_, err = f.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	if !m.Verify(f) {
		panic("Verification of written data failed")
	}
	fmt.Println("Verification OK")
}

func main() {
	if len(os.Args) == 1 {
		fmt.Printf("Usage: %v romfile\n", os.Args[0])
		os.Exit(64)
	}

	f, err := os.OpenFile(os.Args[1], os.O_RDONLY, 0400)
	if err != nil {
		panic(err)
	}

	a := ast.NewAst()
	defer a.Close()

	if a.IsPoweredOn() {
		fmt.Println("Machine is powered on, shutting it off...")
		a.HoldPowerButton(5 * time.Second)
		if a.IsPoweredOn() {
			panic("Unable to turn machine off")
		}
	}
	fmt.Println("Machine is off, it is safe to take SPI master role")

	flashBios(f, a)

	fmt.Println("Flash successful, turning on machine")
	// Do some sanity checking so we don't break stuff
	if a.IsSpiMaster() {
		panic("We are still SPI master, this should not happen")
	}
	if a.IsPoweredOn() {
		panic("We are already powered on, wtf? We might have broken stuff")
	}
	a.HoldPowerButton(1 * time.Second)
}
