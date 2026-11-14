package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	notifySendBinary = "notify-send"
	notifyTimeout    = 10 * time.Second
)

type Desktop struct {
	upIconPath   string
	downIconPath string
}

func NewDesktop(upIcon, downIcon []byte) (*Desktop, error) {
	if _, err := exec.LookPath(notifySendBinary); err != nil {
		return nil, fmt.Errorf("%s not found in PATH", notifySendBinary)
	}

	d := &Desktop{}
	upPath, err := writeTempIcon(upIcon)
	if err != nil {
		return nil, err
	}
	d.upIconPath = upPath
	downPath, err := writeTempIcon(downIcon)
	if err != nil {
		return nil, err
	}
	d.downIconPath = downPath
	return d, nil
}

func writeTempIcon(icon []byte) (string, error) {
	if len(icon) == 0 {
		return "", nil
	}
	f, err := os.CreateTemp("", "pingo-icon-*.svg")
	if err != nil {
		return "", fmt.Errorf("write icon: %w", err)
	}
	if _, err := f.Write(icon); err != nil {
		f.Close()
		return "", fmt.Errorf("write icon: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("write icon: %w", err)
	}
	return f.Name(), nil
}

func (d *Desktop) Notify(ctx context.Context, e Event) error {
	ctx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()

	state, urgency := "UP", "normal"
	icon := d.upIconPath
	if icon == "" {
		icon = "dialog-information"
	}
	if e.Transition == TransitionDown {
		state, urgency = "DOWN", "critical"
		icon = d.downIconPath
		if icon == "" {
			icon = "dialog-error"
		}
	}

	cmd := exec.CommandContext(ctx, notifySendBinary,
		"-a", "pingo",
		"-u", urgency,
		"-i", icon,
		"pingo: "+e.Target+" "+state,
		e.Detail)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", notifySendBinary, err, strings.TrimSpace(string(out)))
	}
	return nil
}
