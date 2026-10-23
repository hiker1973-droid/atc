package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func testImage(t *testing.T, format string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(1, 1, color.RGBA{200, 30, 30, 255})
	var b bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&b, img)
	} else {
		err = jpeg.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCheckLogo(t *testing.T) {
	if ext, err := checkLogo(testImage(t, "png", 256, 256)); err != nil || ext != "png" {
		t.Fatalf("png: %s %v", ext, err)
	}
	if ext, err := checkLogo(testImage(t, "jpg", 300, 200)); err != nil || ext != "jpg" {
		t.Fatalf("jpg: %s %v", ext, err)
	}
	if _, err := checkLogo([]byte("GIF89a not a logo")); err == nil {
		t.Fatal("accepted a non-image")
	}
	if _, err := checkLogo(testImage(t, "png", 8, 8)); err == nil {
		t.Fatal("accepted an 8x8 icon")
	}
	if _, err := checkLogo(make([]byte, maxLogoBytes+1)); err == nil {
		t.Fatal("accepted a file over 2 MB")
	}
}

// The vSFG-7 rigs keep their name with no branding.json; a fresh install is
// Readback ATC until Setup names it. An uploaded logo replaces the other format's.
func TestBrandingDefaultsAndUpload(t *testing.T) {
	root := withRoot(t)
	if got := brandingSnapshot()["name"]; got != "Readback ATC" {
		t.Fatalf("fresh install named %v", got)
	}
	os.WriteFile(filepath.Join(root, "start_towers_caucasus.bat"), []byte("@echo off\r\n"), 0o644)
	if got := brandingSnapshot()["name"]; got != "vSFG-7" {
		t.Fatalf("vSFG-7 pack named %v", got)
	}
	if err := saveLogo(testImage(t, "jpg", 128, 128)); err != nil {
		t.Fatal(err)
	}
	if err := saveLogo(testImage(t, "png", 128, 128)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "branding", "logo.jpg")); err == nil {
		t.Fatal("old logo.jpg left beside the new logo.png")
	}
	snap := brandingSnapshot()
	if snap["customLogo"] != true || readBranding().Logo != "logo.png" {
		t.Fatalf("after upload %v %+v", snap, readBranding())
	}
	b := readBranding()
	b.Name = "VFA-103"
	writeBranding(b)
	if got := brandingSnapshot()["name"]; got != "VFA-103" {
		t.Fatalf("named %v", got)
	}
}
