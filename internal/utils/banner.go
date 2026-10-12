package utils

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type (
	BannerInfo struct {
		Version string
		Addr    net.Addr
		Routes  int
		PID     int
		Startup time.Duration
	}
)

const (
	bannerColour  = "\x1b[38;2;8;124;250m"
	bannerDim     = "\x1b[2m"
	bannerReset   = "\x1b[0m"
	bannerGap     = 3
	bannerLabelAt = 10
)

var (
	bannerArt = []string{
		" ____      _            _____ _     _",
		"|  _ \\ ___| |_ _ __ ___|_   _(_) __| | ___",
		"| |_) / _ \\ __| '__/ _ \\ | | | |/ _` |/ _ \\",
		"|  _ <  __/ |_| | | (_) || | | | (_| |  __/",
		"|_| \\_\\___|\\__|_|  \\___/ |_| |_|\\__,_|\\___|",
	}
)

func WriteBanner(w io.Writer, info BannerInfo) error {
	width := 0
	for _, line := range bannerArt {
		width = max(width, len(line))
	}

	var b strings.Builder
	b.WriteString("\n")

	last := len(bannerArt) - 1
	for i, line := range bannerArt {
		b.WriteString(bannerColour + line + bannerReset)
		if i == last {
			b.WriteString(strings.Repeat(" ", width-len(line)+bannerGap) + info.Version)
		}

		b.WriteString("\n")
	}

	b.WriteString(bannerDim + strings.Repeat("-", width+bannerGap+len(info.Version)) + bannerReset + "\n")

	rows := [][2]string{
		{"Listening", listenURL(info.Addr)},
		{"Handlers", strconv.Itoa(info.Routes)},
		{"PID", strconv.Itoa(info.PID)},
		{"Go", runtime.Version()},
		{"Started in", StartupLabel(info.Startup)},
	}
	for _, row := range rows {
		fmt.Fprintf(&b, "  %-*s %s\n", bannerLabelAt, row[0], row[1])
	}

	b.WriteString("\n")

	_, err := io.WriteString(w, b.String())

	return err
}

func StartupLabel(d time.Duration) string {
	if d >= time.Millisecond {
		return d.Round(time.Millisecond).String()
	}

	return d.Round(time.Microsecond).String()
}

func listenURL(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return "http://" + addr.String()
	}

	local := "http://localhost:" + strconv.Itoa(tcp.Port)
	if tcp.IP.IsUnspecified() {
		return local + " (all interfaces)"
	}

	if tcp.IP.IsLoopback() {
		return local
	}

	return "http://" + addr.String()
}
