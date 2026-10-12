package fracindex

import (
	"fmt"
	"math/big"
	"strings"
)

const (
	MaxKeyLength = 64

	digits    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base      = len(digits)
	smallest  = '0'
	lastDigit = base - 1
)

func Between(lower string, upper string) (string, error) {
	if err := validate(lower); err != nil {
		return "", fmt.Errorf("lower key %q: %w", lower, err)
	}

	if err := validate(upper); err != nil {
		return "", fmt.Errorf("upper key %q: %w", upper, err)
	}

	if upper != "" && upper[len(upper)-1] == smallest {
		return "", fmt.Errorf("upper key %q ends in its smallest digit, so nothing fits below it: %w", upper, ErrInvalidKey)
	}

	if lower != "" && upper != "" && lower >= upper {
		return "", fmt.Errorf("%q is not below %q: %w", lower, upper, ErrNotOrdered)
	}

	if upper == "" {
		return increment(lower), nil
	}

	if lower == "" {
		return decrement(upper), nil
	}

	return midpoint(lower, upper), nil
}

func Spread(n int) []string {
	keys := make([]string, 0, max(n, 0))
	if n <= 0 {
		return keys
	}

	width := 1
	space := big.NewInt(int64(base))
	slots := big.NewInt(int64(n) + 1)
	for space.Cmp(slots) <= 0 {
		width++
		space.Mul(space, big.NewInt(int64(base)))
	}

	value := new(big.Int)
	for i := 1; i <= n; i++ {
		value.Mul(space, big.NewInt(int64(i)))
		value.Quo(value, slots)
		keys = append(keys, encode(value, width))
	}

	return keys
}

func validate(key string) error {
	for _, r := range key {
		if !strings.ContainsRune(digits, r) {
			return fmt.Errorf("%q is not an ordering digit: %w", r, ErrInvalidKey)
		}
	}

	return nil
}

func increment(lower string) string {
	for i := range len(lower) {
		value := strings.IndexByte(digits, lower[i])
		if value < lastDigit {
			return lower[:i] + string(digits[value+1])
		}
	}

	return lower + string(digits[base/2])
}

func decrement(upper string) string {
	for i := range len(upper) {
		value := strings.IndexByte(digits, upper[i])
		if value > 1 {
			return upper[:i] + string(digits[value-1])
		}
	}

	return midpoint("", upper)
}

func midpoint(lower string, upper string) string {
	if upper != "" {
		shared := 0
		for shared < len(upper) && digitAt(lower, shared) == upper[shared] {
			shared++
		}

		if shared > 0 {
			rest := ""
			if shared < len(lower) {
				rest = lower[shared:]
			}

			return upper[:shared] + midpoint(rest, upper[shared:])
		}
	}

	low := 0
	if lower != "" {
		low = strings.IndexByte(digits, lower[0])
	}

	high := base
	if upper != "" {
		high = strings.IndexByte(digits, upper[0])
	}

	if high-low > 1 {
		return string(digits[(low+high)/2])
	}

	if len(upper) > 1 {
		return upper[:1]
	}

	rest := ""
	if lower != "" {
		rest = lower[1:]
	}

	return string(digits[low]) + midpoint(rest, "")
}

func digitAt(key string, position int) byte {
	if position < len(key) {
		return key[position]
	}

	return smallest
}

func encode(value *big.Int, width int) string {
	encoded := []byte(strings.Repeat(string(smallest), width))

	remaining := new(big.Int).Set(value)
	divisor := big.NewInt(int64(base))
	remainder := new(big.Int)
	for i := width - 1; i >= 0 && remaining.Sign() > 0; i-- {
		remaining.QuoRem(remaining, divisor, remainder)
		encoded[i] = digits[remainder.Int64()]
	}

	return strings.TrimRight(string(encoded), string(smallest))
}
