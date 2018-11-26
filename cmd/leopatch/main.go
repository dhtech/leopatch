package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dhtech/leopatch/ast"
)

type partition struct {

}

func main() {
	a := ast.NewAst()
	defer a.Close()

	fmt.Printf(" [*] BMC is %v\n", a.ModelName())

	// Changing the flash and then resuming is not safe, and most likely
	// the user wants to reset to load the new flash anyway
	defer a.ResetCpu()

	a.FreezeCpu()
	fmt.Printf(" [*] CPU now frozen\n")
	defer a.UnfreezeCpu()

	f, err := a.SystemFlash()
	if err != nil {
		log.Fatalf("Unable to acquire flash: %v", err)
	}
	defer f.Close()
	fmt.Printf(" [*] Acquired system flash\n")
	fmt.Printf(" [>] Chip ID: %06x\n", f.Id())

	loc, size := scan(f)
	dump(f, loc, size, "root.bin")

	fmt.Printf(" [*] Root dumped\n")
}

func dump(f ast.Flash, loc int64, size int, fp string) {
	buf := make([]byte, 64*1024)

	df, err := os.OpenFile(fp, os.O_RDWR | os.O_CREATE, 0700)
	if err != nil {
		log.Fatalf("Failed to open dump file: %v", err)
	}
	defer df.Close()
	all := size
	fmt.Printf(" [*] Read progress: 0%%")
	for {
		n, err := f.ReadAt(buf, loc)
		if err != nil {
			log.Fatalf("Failed to dump: %v", err)
		}
		fmt.Printf("\r [*] Read progress: %d%%", 100 - (size * 100 / all))
		loc += int64(len(buf))
		if size < n {
			df.Write(buf[:size])
			break
		} else {
			df.Write(buf[:n])
		}
		size -= n
	}
	fmt.Printf("\r [*] Read complete\n")
}

func scan(f ast.Flash) (int64, int) {
	offset := int64(0x0)

	block := int64(64*1024)
	size := block
	b := make([]byte, size)
	for {
		if offset > 32*1024*1024 {
			break
		}
		n, err := f.ReadAt(b[:size], offset)
		if err != nil {
			log.Fatalf("Read failed: %v", err)
		}

		v := strings.Index(string(b[:n]), "$MODULE$")
		if v == 0 {
			if b[62] == 0xaa && b[63] == 0x55 {
				ssize := int(b[15]) << 24 + int(b[14]) << 16 + int(b[13]) << 8 + int(b[12])
				name := b[24:32]
				fmt.Printf(" [>>] Partition %v @ %x \n", string(name), offset)
				if string(name[:4]) == "root" {
					fmt.Printf(" [*] Found the root, size is %d MiB\n", ssize / 1024 / 1024)
					// TODO(bluecmd): 0x40 seems to be the FMH size, and then 1 erase block
					// seems to be the offset. If this breaks, look at scanning for cramfs
					return offset + 0x10040, ssize - 0x10040
				}
				offset = (offset + int64(ssize)) & 0xffff0000
				continue
			}
		} else if v > -1 {
			// Align to be able to read full header
			size = 128
			offset += int64(v)
			continue
		}

		offset += size
		size = block
	}

	log.Fatal(" [!!] No root partition found")
	return 0, 0
}
