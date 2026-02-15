package domain

import (
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"sort"
)

// SplitNetwork splits a CIDR into subnets of the given prefix size.
func SplitNetwork(cidr string, newSize int) ([]string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	origSize := prefix.Bits()
	addrLen := prefix.Addr().BitLen()

	if newSize < 1 {
		newSize = origSize + 1
	}

	if newSize <= origSize || newSize > addrLen {
		return nil, fmt.Errorf("invalid new size: must be larger than original but not exceed address bit length")
	}

	numSubnets := 1 << (newSize - origSize)
	maxSubnets := 1024
	if numSubnets > maxSubnets {
		return nil, fmt.Errorf("splitting would create %d subnets, current limit is %d due to performance issues", numSubnets, maxSubnets)
	}

	var subnets []string
	addr := prefix.Addr()
	step := big.NewInt(1)
	step.Lsh(step, uint(addrLen-newSize))

	currIP := IPToBigInt(addr)
	isIPv4 := addr.Is4()

	for range 1 << (newSize - origSize) {
		nextPrefix := netip.PrefixFrom(addr, newSize)
		subnets = append(subnets, nextPrefix.String())

		currIP.Add(currIP, step)

		nextAddr := BigIntToAddr(currIP, isIPv4)
		if !nextAddr.IsValid() {
			return nil, fmt.Errorf("failed to convert bigInt %d to IP", currIP)
		}
		addr = nextAddr
	}

	return subnets, nil
}

// SummarizeCIDRs merges a list of contiguous CIDRs into a single summary CIDR.
func SummarizeCIDRs(cidrs []string) (string, error) {
	if len(cidrs) == 0 {
		return "", errors.New("no CIDRs provided")
	}

	var startIPs []*big.Int
	var endIPs []*big.Int
	var minIP, maxIP *big.Int
	var isIPv4 bool

	for i, cidrStr := range cidrs {
		prefix, err := netip.ParsePrefix(cidrStr)
		if err != nil {
			return "", fmt.Errorf("invalid CIDR %s: %w", cidrStr, err)
		}

		if i == 0 {
			isIPv4 = prefix.Addr().Is4()
		} else if isIPv4 != prefix.Addr().Is4() {
			return "", errors.New("mixed IPv4 and IPv6 addresses are not supported")
		}

		startIP := IPToBigInt(prefix.Masked().Addr())
		endIP := IPToBigInt(LastAddr(prefix))

		startIPs = append(startIPs, startIP)
		endIPs = append(endIPs, endIP)

		if i == 0 || startIP.Cmp(minIP) < 0 {
			minIP = startIP
		}
		if i == 0 || endIP.Cmp(maxIP) > 0 {
			maxIP = endIP
		}
	}

	indices := make([]int, len(startIPs))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return startIPs[indices[i]].Cmp(startIPs[indices[j]]) < 0
	})

	sortedStartIPs := make([]*big.Int, len(startIPs))
	sortedEndIPs := make([]*big.Int, len(endIPs))
	for i, idx := range indices {
		sortedStartIPs[i] = startIPs[idx]
		sortedEndIPs[i] = endIPs[idx]
	}

	currentIP := new(big.Int).Set(minIP)
	one := big.NewInt(1)

	for i := range sortedStartIPs {
		if sortedStartIPs[i].Cmp(currentIP) > 0 {
			return "", errors.New("CIDRs are not contiguous; gaps detected")
		}
		if sortedStartIPs[i].Cmp(currentIP) <= 0 && sortedEndIPs[i].Cmp(currentIP) >= 0 {
			currentIP = new(big.Int).Add(sortedEndIPs[i], one)
		}
	}
	if new(big.Int).Sub(currentIP, one).Cmp(maxIP) != 0 {
		return "", errors.New("CIDRs do not cover a contiguous range")
	}

	totalAddresses := new(big.Int).Add(new(big.Int).Sub(maxIP, minIP), one)
	if !isPowerOfTwo(totalAddresses) {
		return "", errors.New("total number of addresses is not a power of two; cannot summarize into a single CIDR without including extra addresses")
	}

	if new(big.Int).Mod(minIP, totalAddresses).Cmp(big.NewInt(0)) != 0 {
		return "", errors.New("network address is not aligned; cannot summarize into a single CIDR without including extra addresses")
	}

	var addressBits int
	if isIPv4 {
		addressBits = 32
	} else {
		addressBits = 128
	}
	prefixLength := addressBits - log2BigInt(totalAddresses)

	summarizedIP := BigIntToAddr(minIP, isIPv4)

	summarizedPrefix := netip.PrefixFrom(summarizedIP, prefixLength)
	return summarizedPrefix.String(), nil
}

// FindSummarizableRange finds the largest set of contiguous CIDRs containing
// the element at index that can be summarized into a single CIDR.
func FindSummarizableRange(cidrs []string, index int) (bool, []string, string) {
	maxSize := len(cidrs)
	for size := maxSize; size >= 2; size-- {
		for start := index - size + 1; start <= index; start++ {
			end := start + size - 1
			if start < 0 || end >= len(cidrs) {
				continue
			}
			if start <= index && index <= end {
				subCIDRs := cidrs[start : end+1]
				summarizedCIDR, err := SummarizeCIDRs(subCIDRs)
				if err == nil {
					return true, subCIDRs, summarizedCIDR
				}
			}
		}
	}
	return false, nil, ""
}

// ---------- Big-integer IP helpers ----------

// IPToBigInt converts a netip.Addr to a big.Int.
func IPToBigInt(addr netip.Addr) *big.Int {
	ip := addr.AsSlice()
	return new(big.Int).SetBytes(ip)
}

// BigIntToAddr converts a big.Int back to a netip.Addr.
func BigIntToAddr(i *big.Int, isIPv4 bool) netip.Addr {
	var ipLen int
	if isIPv4 {
		ipLen = 4
	} else {
		ipLen = 16
	}
	ipBytes := i.Bytes()
	if len(ipBytes) < ipLen {
		padding := make([]byte, ipLen-len(ipBytes))
		ipBytes = append(padding, ipBytes...)
	} else if len(ipBytes) > ipLen {
		ipBytes = ipBytes[len(ipBytes)-ipLen:]
	}
	addr, ok := netip.AddrFromSlice(ipBytes)
	if !ok {
		return netip.Addr{}
	}
	return addr
}

// LastAddr returns the last address in a prefix.
func LastAddr(prefix netip.Prefix) netip.Addr {
	addr := prefix.Masked().Addr()
	addrInt := IPToBigInt(addr)
	var size big.Int
	if addr.Is4() {
		size.Exp(big.NewInt(2), big.NewInt(32-int64(prefix.Bits())), nil)
	} else {
		size.Exp(big.NewInt(2), big.NewInt(128-int64(prefix.Bits())), nil)
	}
	size.Sub(&size, big.NewInt(1))
	endInt := new(big.Int).Add(addrInt, &size)
	return BigIntToAddr(endInt, addr.Is4())
}

func log2BigInt(n *big.Int) int {
	bits := n.BitLen()
	if bits == 0 {
		return 0
	}
	return bits - 1
}

func isPowerOfTwo(n *big.Int) bool {
	if n.Sign() <= 0 {
		return false
	}
	one := big.NewInt(1)
	tmp := new(big.Int).Sub(n, one)
	return new(big.Int).And(n, tmp).Cmp(big.NewInt(0)) == 0
}
