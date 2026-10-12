package dronebl

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

const (
	zone = "dnsbl.dronebl.org"

	minClass = 2
	maxClass = 255
)

func routable(ip string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return netip.Addr{}, false
	}

	addr = addr.Unmap()
	if addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
		return netip.Addr{}, false
	}

	return addr, true
}

func queryName(addr netip.Addr) string {
	if addr.Is4() {
		octets := addr.As4()

		return fmt.Sprintf("%d.%d.%d.%d.%s", octets[3], octets[2], octets[1], octets[0], zone)
	}

	octets := addr.As16()
	labels := strings.Split(hex.EncodeToString(octets[:]), "")
	slices.Reverse(labels)

	return strings.Join(append(labels, zone), ".")
}

func classesFrom(answers []netip.Addr) []int {
	classes := make([]int, 0, len(answers))
	for _, answer := range answers {
		answer = answer.Unmap()
		if !answer.Is4() {
			continue
		}

		octets := answer.As4()
		if octets[0] == 127 && octets[3] >= minClass {
			classes = append(classes, int(octets[3]))
		}
	}

	return classes
}

func parseAllowlist(raw string) ([]netip.Prefix, error) {
	var (
		prefixes []netip.Prefix
		errs     []error
	)

	for field := range strings.SplitSeq(raw, ",") {
		entry := strings.TrimSpace(field)
		if entry == "" {
			continue
		}

		prefix, err := parseAllowlistEntry(entry)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		prefixes = append(prefixes, prefix)
	}

	return prefixes, errors.Join(errs...)
}

func parseAllowlistEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("dronebl_allowlist entry %q is not a prefix: %w", entry, err)
		}

		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("dronebl_allowlist entry %q is not an address: %w", entry, err)
	}

	addr = addr.Unmap()

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func allowlisted(prefixes []netip.Prefix, addr netip.Addr) bool {
	return slices.ContainsFunc(prefixes, func(prefix netip.Prefix) bool {
		return prefix.Contains(addr)
	})
}

func parseIgnoredClasses(raw string) (map[int]struct{}, error) {
	var errs []error

	ignored := make(map[int]struct{})
	for field := range strings.SplitSeq(raw, ",") {
		entry := strings.TrimSpace(field)
		if entry == "" {
			continue
		}

		class, err := strconv.Atoi(entry)
		if err != nil {
			errs = append(errs, fmt.Errorf("dronebl_ignored_classes entry %q is not a whole number: %w", entry, err))
			continue
		}

		if class < minClass || class > maxClass {
			errs = append(errs, fmt.Errorf("dronebl_ignored_classes entry %d is not a class from %d to %d", class, minClass, maxClass))
			continue
		}

		ignored[class] = struct{}{}
	}

	return ignored, errors.Join(errs...)
}
