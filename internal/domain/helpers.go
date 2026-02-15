package domain

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// ---------- Parsing ----------

// ParsePositiveIntID parses a string as a positive integer.
func ParsePositiveIntID(input string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, fmt.Errorf("must be a positive integer: %w", err)
	}
	if value < 1 {
		return 0, fmt.Errorf("must be a positive integer, got %d", value)
	}
	return value, nil
}

// ParseVLANListCSV parses a comma-separated list of VLAN IDs.
func ParseVLANListCSV(text string) ([]int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	parts := strings.Split(text, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid VLAN ID %q: %w", part, err)
		}
		if id < 1 || id > 4094 {
			return nil, fmt.Errorf("VLAN ID %d out of range 1-4094", id)
		}
		result = append(result, id)
	}
	return result, nil
}

// ParseOptionalVLANID parses an optional VLAN ID, returning 0 for empty input.
func ParseOptionalVLANID(text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	id, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("invalid VLAN ID %q: %w", text, err)
	}
	if id < 1 || id > 4094 {
		return 0, fmt.Errorf("VLAN ID %d out of range 1-4094", id)
	}
	return id, nil
}

// ParseOptionalIntField parses an optional integer field, returning 0 for empty input.
func ParseOptionalIntField(text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}
	return strconv.Atoi(text)
}

// ParseTaggedMode parses a tagged VLAN mode string.
func ParseTaggedMode(text string) TaggedVLANMode {
	text = strings.TrimSpace(text)
	switch {
	case strings.EqualFold(text, string(TaggedVLANModeAllowAll)):
		return TaggedVLANModeAllowAll
	case strings.EqualFold(text, string(TaggedVLANModeBlockAll)):
		return TaggedVLANModeBlockAll
	case strings.EqualFold(text, string(TaggedVLANModeCustom)):
		return TaggedVLANModeCustom
	default:
		return TaggedVLANModeNone
	}
}

// ---------- Sorting ----------

// CompareNaturalNumberOrder compares two strings using natural number ordering.
func CompareNaturalNumberOrder(left, right string) int {
	l := []rune(strings.ToLower(left))
	r := []rune(strings.ToLower(right))
	li, ri := 0, 0

	for li < len(l) && ri < len(r) {
		if isDigitRune(l[li]) && isDigitRune(r[ri]) {
			lnStart, rnStart := li, ri
			for li < len(l) && isDigitRune(l[li]) {
				li++
			}
			for ri < len(r) && isDigitRune(r[ri]) {
				ri++
			}

			ln := trimLeadingZeroes(string(l[lnStart:li]))
			rn := trimLeadingZeroes(string(r[rnStart:ri]))

			if c := cmp.Compare(len(ln), len(rn)); c != 0 {
				return c
			}
			if c := cmp.Compare(ln, rn); c != 0 {
				return c
			}
			if c := cmp.Compare(li-lnStart, ri-rnStart); c != 0 {
				return c
			}
			continue
		}

		if c := cmp.Compare(l[li], r[ri]); c != 0 {
			return c
		}
		li++
		ri++
	}

	if c := cmp.Compare(len(l)-li, len(r)-ri); c != 0 {
		return c
	}

	return cmp.Compare(left, right)
}

func trimLeadingZeroes(value string) string {
	trimmed := strings.TrimLeft(value, "0")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

func isDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}

// ---------- Identifiers ----------

// CIDRToIdentifier converts a CIDR to a hex-encoded file name identifier.
func CIDRToIdentifier(cidr string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	var ipBytes []byte
	if ip4 := ip.To4(); ip4 != nil {
		ipBytes = ip4
	} else if ip16 := ip.To16(); ip16 != nil {
		ipBytes = ip16
	} else {
		return "", fmt.Errorf("invalid IP address in CIDR")
	}

	ipHex := hex.EncodeToString(ipBytes)
	prefixLen, _ := ipNet.Mask.Size()
	identifier := fmt.Sprintf("%s_%d", ipHex, prefixLen)
	return identifier, nil
}

// IPToIdentifier converts an IP address to a hex-encoded file name identifier.
func IPToIdentifier(ipStr string) (string, error) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", fmt.Errorf("invalid IP: %w", err)
	}

	return hex.EncodeToString(addr.AsSlice()), nil
}
