package dockersquasher

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type tarSquasher interface {
	Extract(archive io.ReadCloser, outdir string) error
	Squash(indir string, outfile string) error
}

type tarSquasherImpl struct{}

func (tsi tarSquasherImpl) Extract(archive io.ReadCloser, outdir string) error {
	_, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("the 'tar' command is unavailable. cannot extract layers")
	}
	tarExtract := exec.Command("tar", "-xpzf", "-", "-C", outdir)
	tarErr, _ := tarExtract.StderrPipe()
	tarOut, _ := tarExtract.StdoutPipe()
	tarIn, _ := tarExtract.StdinPipe()

	tarStdout := new(strings.Builder)
	tarStderr := new(strings.Builder)

	err = tarExtract.Start()
	if err != nil {
		return fmt.Errorf("failed to start tar - is tar not functional? %w", err)
	}
	go func() {
		io.Copy(tarStdout, tarOut)
	}()
	go func() {
		io.Copy(tarStderr, tarErr)
	}()

	_, err = io.Copy(tarIn, archive)
	if err != nil {
		archive.Close()
		tarIn.Close()
		tarExtract.Wait()
		return fmt.Errorf("failed to copy tar data into tar - something bad has happened. %w", err)
	}
	archive.Close()
	tarIn.Close()
	err = tarExtract.Wait()
	if err != nil {
		return fmt.Errorf("failed to extract tar archive: %s %s %w", tarStdout.String(), tarStderr.String(), err)
	}
	return nil
}

func (tsi tarSquasherImpl) Squash(indir string, outfile string) error {
	_, err := exec.LookPath("mksquashfs")
	if err != nil {
		return fmt.Errorf("the 'mksquashfs' command is unavailable. cannot ")
	}

	mksqfs := exec.Command("mksquashfs", indir, outfile)

	out, err := mksqfs.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to squash the rootfs. Output: %s. %w", string(out), err)
	}
	return nil
}
