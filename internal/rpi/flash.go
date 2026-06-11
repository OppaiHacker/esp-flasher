package rpi

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// imageStream provides an uncompressed image data stream along with
// the known size (total == -1 when the size is unknown).
type imageStream struct {
	r       io.Reader
	total   int64
	closers []io.Closer
	cmd     *exec.Cmd // xz process, if used
}

// Close closes stream resources and waits for an optional xz process.
// The xz process error is returned so as not to overlook a corrupted archive.
func (s *imageStream) Close() error {
	var firstErr error
	// Close in reverse order: first the readers, then the file.
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.cmd != nil {
		if err := s.cmd.Wait(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("xz decompression failed: %w", err)
		}
	}
	return firstErr
}

// openImageStream opens an image for reading, decompressing it on the fly
// depending on the extension (.xz/.gz/.zip or plain .img/.iso).
func openImageStream(img string) (*imageStream, error) {
	lower := strings.ToLower(img)

	// Read ZIP archive with the standard library — we know the entry size.
	if strings.HasSuffix(lower, ".zip") {
		zr, err := zip.OpenReader(img)
		if err != nil {
			return nil, fmt.Errorf("cannot open archive %s: %w", filepath.Base(img), err)
		}
		entry := zipImageEntry(zr)
		if entry == nil {
			zr.Close()
			return nil, fmt.Errorf("archive %s does not contain an .img or .iso file", filepath.Base(img))
		}
		rc, err := entry.Open()
		if err != nil {
			zr.Close()
			return nil, fmt.Errorf("cannot read entry %s from archive: %w", entry.Name, err)
		}
		return &imageStream{
			r:       rc,
			total:   int64(entry.UncompressedSize64),
			closers: []io.Closer{zr, rc},
		}, nil
	}

	f, err := os.Open(img)
	if err != nil {
		return nil, fmt.Errorf("cannot open image %s: %w", img, err)
	}

	switch {
	case strings.HasSuffix(lower, ".xz"):
		// Decompress on the fly with `xz -dc`.
		if _, err := exec.LookPath("xz"); err != nil {
			f.Close()
			return nil, fmt.Errorf("missing 'xz' program — install xz-utils package: %w", err)
		}
		cmd := exec.Command("xz", "-dc")
		cmd.Stdin = f
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("cannot create pipe for xz: %w", err)
		}
		if err := cmd.Start(); err != nil {
			f.Close()
			return nil, fmt.Errorf("cannot run xz program: %w", err)
		}
		return &imageStream{
			r:       stdout,
			total:   -1,
			closers: []io.Closer{f, stdout},
			cmd:     cmd,
		}, nil

	case strings.HasSuffix(lower, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("file %s is not a valid gzip archive: %w", filepath.Base(img), err)
		}
		return &imageStream{r: gz, total: -1, closers: []io.Closer{f, gz}}, nil

	default:
		// Plain image — we know the size from the filesystem.
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("cannot read image size %s: %w", img, err)
		}
		return &imageStream{r: f, total: info.Size(), closers: []io.Closer{f}}, nil
	}
}

// collectMountpoints recursively collects all mountpoints
// for a device and its partitions from lsblk output.
func collectMountpoints(devs []lsblkDevice, into *[]string) {
	for _, d := range devs {
		for _, mp := range d.Mountpoints {
			if mp != nil && *mp != "" && *mp != "[SWAP]" {
				*into = append(*into, *mp)
			}
		}
		collectMountpoints(d.Children, into)
	}
}

// unmountAll unmounts all mounted partitions of the device.
func unmountAll(device string, log func(string)) error {
	parsed, err := runLsblk("-J", "-o", "NAME,MOUNTPOINTS", device)
	if err != nil {
		return fmt.Errorf("cannot check mountpoints for %s: %w", device, err)
	}
	var mountpoints []string
	collectMountpoints(parsed.Blockdevices, &mountpoints)
	for _, mp := range mountpoints {
		if _, err := runLogged(log, "sudo", "umount", mp); err != nil {
			return fmt.Errorf("cannot unmount %s: %w", mp, err)
		}
	}
	return nil
}

// FlashImage writes the image to the device.
//
// SECURITY: before writing, it checks again via lsblk whether the
// device is removable media — otherwise it refuses.
// First, it unmounts all mounted partitions of the device.
// Writing is done via `sudo dd of=DEVICE bs=4M conv=fsync`,
// and the image (also .xz/.gz/.zip, decompressed on the fly) is streamed
// to dd's stdin with byte counting for progress(done, total) — total is
// the uncompressed size if known, otherwise -1. Finally, `sync` is
// executed. Every external command is logged via log().
func FlashImage(img, device string, progress func(done, total int64), log func(string)) error {
	// 1. Security check: removable media only.
	drives, err := ListRemovableDrives()
	if err != nil {
		return fmt.Errorf("cannot verify device %s: %w", device, err)
	}
	removable := false
	for _, d := range drives {
		if d.Device == device {
			removable = true
			break
		}
	}
	if !removable {
		return fmt.Errorf("device %s is not removable media — refusing to write to prevent damaging internal drive", device)
	}

	// 2. Unmount all partitions of the device.
	if err := unmountAll(device, log); err != nil {
		return err
	}

	// 3. Stream image to dd's stdin.
	stream, err := openImageStream(img)
	if err != nil {
		return err
	}
	cr := &countingReader{r: stream.r, total: stream.total, progress: progress}

	ddArgs := []string{"dd", "of=" + device, "bs=4M", "conv=fsync", "status=none"}
	if log != nil {
		log("$ sudo " + strings.Join(ddArgs, " "))
	}
	cmd := exec.Command("sudo", ddArgs...)
	cmd.Stdin = cr
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		stream.Close()
		return fmt.Errorf("writing image to %s failed: %w (%s)",
			device, err, strings.TrimSpace(stderr.String()))
	}
	if err := stream.Close(); err != nil {
		return fmt.Errorf("reading image %s failed: %w", filepath.Base(img), err)
	}
	if progress != nil {
		progress(cr.done, cr.total)
	}

	// 4. Flush system buffers.
	if _, err := runLogged(log, "sync"); err != nil {
		return fmt.Errorf("sync command failed: %w", err)
	}
	return nil
}

// VerifyFlash compares the SHA-256 sum of the image with the first len(image)
// bytes of the device (read via `sudo dd if=DEVICE` streamed
// to Go). Works only for uncompressed .img; for compressed images
// it returns (true, nil) with an annotation in the log.
func VerifyFlash(img, device string, progress func(done, total int64), log func(string)) (bool, error) {
	logf := func(s string) {
		if log != nil {
			log(s)
		}
	}
	if isCompressedName(img) {
		logf("Image is compressed — skipping checksum verification.")
		return true, nil
	}

	f, err := os.Open(img)
	if err != nil {
		return false, fmt.Errorf("cannot open image %s: %w", img, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("cannot read image size %s: %w", img, err)
	}
	size := info.Size()
	if size == 0 {
		return false, fmt.Errorf("image %s is empty", filepath.Base(img))
	}

	// Progress covers two passes: hashing the image and reading the media.
	total := 2 * size

	// 1. Checksum of the image file.
	imgHash := sha256.New()
	if _, err := io.Copy(imgHash, &countingReader{r: f, total: total, progress: progress}); err != nil {
		return false, fmt.Errorf("hashing image failed: %w", err)
	}

	// 2. Checksum of the first `size` bytes of the device.
	const blockSize = 4 << 20
	count := (size + blockSize - 1) / blockSize
	ddArgs := []string{"dd", "if=" + device, "bs=4M", fmt.Sprintf("count=%d", count), "status=none"}
	if log != nil {
		log("$ sudo " + strings.Join(ddArgs, " "))
	}
	cmd := exec.Command("sudo", ddArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("cannot create pipe for dd: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("cannot run dd to read %s: %w", device, err)
	}

	devHash := sha256.New()
	cr := &countingReader{r: io.LimitReader(stdout, size), done: size, last: size, total: total, progress: progress}
	read, copyErr := io.Copy(devHash, cr)
	// Drain the rest of the dd block so the process can exit cleanly.
	io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	if copyErr != nil {
		return false, fmt.Errorf("reading device %s failed: %w", device, copyErr)
	}
	if waitErr != nil {
		return false, fmt.Errorf("reading device %s failed: %w (%s)",
			device, waitErr, strings.TrimSpace(stderr.String()))
	}
	if read < size {
		return false, fmt.Errorf("device %s contains less data (%s) than the image (%s)",
			device, HumanSize(read), HumanSize(size))
	}
	if progress != nil {
		progress(total, total)
	}

	match := bytes.Equal(imgHash.Sum(nil), devHash.Sum(nil))
	if match {
		logf("Verification successful — checksums match.")
	} else {
		logf("Verification failed — checksums differ!")
	}
	return match, nil
}
