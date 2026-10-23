// winicon turns an .ico into a Go .syso resource file, so the program built
// from that folder carries the icon (Explorer, the taskbar, shortcuts). No
// rc.exe or windres needed. Operator 2026-10-23: "make the .ico file for the
// exe logo".
//
//	go run ./cmd/winicon cmd/winicon/readback.ico cmd/installer/rsrc_windows_amd64.syso
//	go run ./cmd/winicon cmd/winicon/readback.ico cmd/launcher/rsrc_windows_amd64.syso
//
// The .syso is an amd64 COFF object with one .rsrc section: RT_ICON images
// 1..n and RT_GROUP_ICON 1 pointing at them (the first group icon is the one
// Windows shows for the exe). The Go linker relocates the data RVAs.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	langEnUS    = 0x0409
)

type icoImage struct {
	w, h     byte
	colors   byte
	planes   uint16
	bitCount uint16
	data     []byte
}

func readICO(path string) ([]icoImage, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(d) < 6 || binary.LittleEndian.Uint16(d[2:]) != 1 {
		return nil, fmt.Errorf("%s is not an .ico", path)
	}
	n := int(binary.LittleEndian.Uint16(d[4:]))
	var out []icoImage
	for i := 0; i < n; i++ {
		e := d[6+16*i:]
		if len(e) < 16 {
			return nil, fmt.Errorf("%s: truncated directory", path)
		}
		size := binary.LittleEndian.Uint32(e[8:])
		off := binary.LittleEndian.Uint32(e[12:])
		if int(off)+int(size) > len(d) {
			return nil, fmt.Errorf("%s: image %d runs past the end", path, i+1)
		}
		img := icoImage{w: e[0], h: e[1], colors: e[2], planes: binary.LittleEndian.Uint16(e[4:]),
			bitCount: binary.LittleEndian.Uint16(e[6:]), data: d[off : off+size]}
		if img.planes == 0 {
			img.planes = 1
		}
		out = append(out, img)
	}
	return out, nil
}

// leaf is one resource: its type, ID and bytes.
type leaf struct {
	typ, id uint32
	data    []byte
}

func buildSyso(imgs []icoImage) []byte {
	var leaves []leaf
	group := new(bytes.Buffer)
	binary.Write(group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(imgs))})
	for i, img := range imgs {
		id := uint32(i + 1)
		leaves = append(leaves, leaf{rtIcon, id, img.data})
		group.Write([]byte{img.w, img.h, img.colors, 0})
		binary.Write(group, binary.LittleEndian, img.planes)
		binary.Write(group, binary.LittleEndian, img.bitCount)
		binary.Write(group, binary.LittleEndian, uint32(len(img.data)))
		binary.Write(group, binary.LittleEndian, uint16(id))
	}
	leaves = append(leaves, leaf{rtGroupIcon, 1, group.Bytes()})

	// Layout: root dir, one type dir per type, one language dir per leaf,
	// the data entries, then the data.
	types := []uint32{rtIcon, rtGroupIcon} // IDs in ascending order
	byType := map[uint32][]int{}
	for i, l := range leaves {
		byType[l.typ] = append(byType[l.typ], i)
	}
	dirSize := func(n int) int { return 16 + 8*n }
	off := dirSize(len(types))
	typeDirOff := map[uint32]int{}
	for _, t := range types {
		typeDirOff[t] = off
		off += dirSize(len(byType[t]))
	}
	langDirOff := make([]int, len(leaves))
	for i := range leaves {
		langDirOff[i] = off
		off += dirSize(1)
	}
	entryOff := make([]int, len(leaves))
	for i := range leaves {
		entryOff[i] = off
		off += 16
	}
	dataOff := make([]int, len(leaves))
	for i, l := range leaves {
		off = (off + 7) &^ 7
		dataOff[i] = off
		off += len(l.data)
	}
	sec := make([]byte, (off+7)&^7)
	le := binary.LittleEndian
	dir := func(at, n int) {
		le.PutUint16(sec[at+14:], uint16(n)) // NumberOfIdEntries
	}
	ent := func(at int, id uint32, target int, sub bool) {
		le.PutUint32(sec[at:], id)
		v := uint32(target)
		if sub {
			v |= 0x80000000
		}
		le.PutUint32(sec[at+4:], v)
	}
	dir(0, len(types))
	for i, t := range types {
		ent(16+8*i, t, typeDirOff[t], true)
		dir(typeDirOff[t], len(byType[t]))
		for j, li := range byType[t] {
			ent(typeDirOff[t]+16+8*j, leaves[li].id, langDirOff[li], true)
		}
	}
	var relocs []int // offsets of the data-entry RVAs
	for i, l := range leaves {
		dir(langDirOff[i], 1)
		ent(langDirOff[i]+16, langEnUS, entryOff[i], false)
		le.PutUint32(sec[entryOff[i]:], uint32(dataOff[i])) // RVA, relocated by the linker
		le.PutUint32(sec[entryOff[i]+4:], uint32(len(l.data)))
		relocs = append(relocs, entryOff[i])
		copy(sec[dataOff[i]:], l.data)
	}

	// COFF: header, one section header, the section, relocations, one
	// symbol (the section), an empty string table.
	const hdr, shdr = 20, 40
	rawPtr := hdr + shdr
	relPtr := rawPtr + len(sec)
	symPtr := relPtr + 10*len(relocs)
	out := new(bytes.Buffer)
	w := func(v any) { binary.Write(out, le, v) }
	w(uint16(0x8664)) // AMD64
	w(uint16(1))
	w(uint32(0))
	w(uint32(symPtr))
	w(uint32(1))
	w(uint16(0))
	w(uint16(0))
	out.WriteString(".rsrc\x00\x00\x00")
	w(uint32(0))
	w(uint32(0))
	w(uint32(len(sec)))
	w(uint32(rawPtr))
	w(uint32(relPtr))
	w(uint32(0))
	w(uint16(len(relocs)))
	w(uint16(0))
	w(uint32(0x40000040)) // initialized data, readable
	out.Write(sec)
	for _, r := range relocs {
		w(uint32(r))
		w(uint32(0))      // symbol 0: .rsrc
		w(uint16(0x0003)) // IMAGE_REL_AMD64_ADDR32NB
	}
	out.WriteString(".rsrc\x00\x00\x00")
	w(uint32(0)) // value
	w(uint16(1)) // section 1
	w(uint16(0)) // type
	w(uint8(3))  // IMAGE_SYM_CLASS_STATIC
	w(uint8(0))  // no aux records
	w(uint32(4)) // string table: just its size
	return out.Bytes()
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: winicon <in.ico> <out.syso>")
		os.Exit(2)
	}
	imgs, err := readICO(os.Args[1])
	if err == nil {
		err = os.WriteFile(os.Args[2], buildSyso(imgs), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d icon images\n", os.Args[2], len(imgs))
}
