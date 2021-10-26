package packetfilter

import (
	"fmt"
	"os/exec"
	"strings"
)

type tcHelperImpl struct{}

type execTcHelper func(args ...string) (string, error)

func execHelper(args ...string) (string, error) {
	cmd := exec.Command("tc", args...)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (tc *tcHelperImpl) EnsureQdiscClsact(ifce string) error {
	return tc.ensureQdiscClsact(ifce, execHelper)
}

func (tc *tcHelperImpl) ensureQdiscClsact(ifce string, execTc execTcHelper) error {
	res, err := execTc("qdisc", "show", "dev", ifce)
	if err != nil {
		return fmt.Errorf("failed to check qdisc status - interface nonexistant? %w", err)
	}
	if strings.Contains(res, "clsact") {
		// clsact already in place!
		return nil
	} else if strings.Contains(res, "fq_codel") {
		// some TAP devices get created with fq_codel for reasons beyond me.
		// Replace it with a noqeueu
		//tc qdisc replace dev tap1 root noqueue
		_, err := execTc("qdisc", "replace", "dev", ifce, "root", "noqueue")
		if err != nil {
			return fmt.Errorf("failed to remove qdisc? %w", err)
		}
		res, err = execTc("qdisc", "show", "dev", ifce)
		if err != nil {
			return fmt.Errorf("failed to check qdisc status - interface nonexistant? %w", err)
		}
	}
	if strings.Contains(res, "noqueue") {
		_, err = execTc("qdisc", "add", "dev", ifce, "clsact")
		if err != nil {
			return fmt.Errorf("failed to add clsact classifier to %s: %w", ifce, err)
		}
		return nil
	}

	return fmt.Errorf("interface %s already has novel queuing configured. Cannot add clsact classifier", ifce)
}

func (tc *tcHelperImpl) LoadBPFIngress(ifce string, path string) error {
	return tc.loadBPFIngress(ifce, path, execHelper)
}

func (tc *tcHelperImpl) loadBPFIngress(ifce string, path string, execTc execTcHelper) error {
	// Remove anything that's in place...
	res, err := execTc("filter", "del", "dev", ifce, "ingress")
	if err != nil {
		return fmt.Errorf("failed to remove existing filters: %s: %w", res, err)
	}
	// and then add ours:
	res, err = execTc("filter", "add", "dev", ifce, "ingress", "bpf", "da", "obj", path, "sec", "ingress")
	if err != nil {
		return fmt.Errorf("could not insert filter (wrong path?) %s %w", res, err)
	}
	return nil
}
