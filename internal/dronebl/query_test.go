package dronebl

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutable(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
		ok   bool
	}{
		{name: "a public ipv4 address", ip: "190.114.41.165", want: "190.114.41.165", ok: true},
		{name: "a public ipv6 address", ip: "2600:387:15:4015::4", want: "2600:387:15:4015::4", ok: true},
		{name: "an ipv4 mapped ipv6 address is treated as ipv4", ip: "::ffff:8.8.4.4", want: "8.8.4.4", ok: true},
		{name: "surrounding space is ignored", ip: " 1.1.1.1 ", want: "1.1.1.1", ok: true},
		{name: "a private address is never sent to a third party", ip: "192.168.1.5", ok: false},
		{name: "a private ipv6 address", ip: "fd00::1", ok: false},
		{name: "loopback", ip: "127.0.0.1", ok: false},
		{name: "ipv6 loopback", ip: "::1", ok: false},
		{name: "an ipv6 link local address", ip: "fe80::1", ok: false},
		{name: "an ipv4 link local address", ip: "169.254.10.10", ok: false},
		{name: "the unspecified address", ip: "0.0.0.0", ok: false},
		{name: "junk", ip: "not-an-ip", ok: false},
		{name: "an empty address", ip: "", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			ip := tc.ip

			// when
			addr, ok := routable(ip)

			// then
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				assert.Equal(t, tc.want, addr.String())
			}
		})
	}
}

func TestQueryName(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{name: "a symmetric ipv4 address", ip: "1.1.1.1", want: "1.1.1.1.dnsbl.dronebl.org"},
		{name: "an asymmetric ipv4 address proves the order", ip: "190.114.41.165", want: "165.41.114.190.dnsbl.dronebl.org"},
		{name: "ipv6 expands to reversed nibbles", ip: "2001:4860:4860::8888", want: "8.8.8.8.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.6.8.4.0.6.8.4.1.0.0.2.dnsbl.dronebl.org"},
		{name: "a listed ipv6 member address", ip: "2600:387:15:4015::4", want: "4.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.5.1.0.4.5.1.0.0.7.8.3.0.0.0.6.2.dnsbl.dronebl.org"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			addr := netip.MustParseAddr(tc.ip)

			// when
			got := queryName(addr)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestQueryName_IPv6HasOneLabelPerNibble(t *testing.T) {
	// given any ipv6 address
	addr := netip.MustParseAddr("2402:800:6378:8672:785a:5340:9063:fa81")

	// when
	name := queryName(addr)

	// then RFC 5782 asks for one label per nibble, then the three labels of the zone
	assert.Equal(t, 34, strings.Count(name, "."))
}

func TestClassesFrom(t *testing.T) {
	tests := []struct {
		name    string
		answers []string
		want    []int
	}{
		{name: "a listing yields its class", answers: []string{"127.0.0.13"}, want: []int{13}},
		{name: "the rfc test entry is not a listing", answers: []string{"127.0.0.1"}, want: []int{}},
		{name: "several classes are all reported", answers: []string{"127.0.0.3", "127.0.0.9"}, want: []int{3, 9}},
		{name: "an answer outside 127/8 is ignored", answers: []string{"10.0.0.5"}, want: []int{}},
		{name: "an ipv6 answer is ignored", answers: []string{"2001:db8::13"}, want: []int{}},
		{name: "no answers means clean", answers: nil, want: []int{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			answers := make([]netip.Addr, 0, len(tc.answers))
			for _, raw := range tc.answers {
				answers = append(answers, netip.MustParseAddr(raw))
			}

			// when
			got := classesFrom(answers)

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseIgnoredClasses(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []int
		wantErr bool
	}{
		{name: "an empty setting ignores nothing", raw: "", want: nil},
		{name: "only spaces and commas ignore nothing", raw: " , ,", want: nil},
		{name: "one class", raw: "13", want: []int{13}},
		{name: "several classes with spaces", raw: " 13 , 18 ", want: []int{13, 18}},
		{name: "junk is refused but the sane classes survive", raw: "13, nonsense", want: []int{13}, wantErr: true},
		{name: "the rfc test class is refused", raw: "1, 13", want: []int{13}, wantErr: true},
		{name: "a class past 255 is refused", raw: "999", want: nil, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			raw := tc.raw

			// when
			ignored, err := parseIgnoredClasses(raw)

			// then
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Len(t, ignored, len(tc.want))
			for _, class := range tc.want {
				assert.Contains(t, ignored, class)
			}
		})
	}
}

func TestParseAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "an empty setting allows nothing", raw: "", want: nil},
		{name: "a bare ipv4 address becomes a single host prefix", raw: "203.0.113.4", want: []string{"203.0.113.4/32"}},
		{name: "a bare ipv6 address becomes a single host prefix", raw: "2600:387:15:4015::4", want: []string{"2600:387:15:4015::4/128"}},
		{name: "a prefix keeps its length and loses its host bits", raw: "203.0.113.9/24", want: []string{"203.0.113.0/24"}},
		{name: "several entries with spaces", raw: " 10.0.0.1 , 203.0.113.0/24 ", want: []string{"10.0.0.1/32", "203.0.113.0/24"}},
		{name: "junk is refused but the sane entries survive", raw: "not-a-cidr, 203.0.113.4", want: []string{"203.0.113.4/32"}, wantErr: true},
		{name: "a prefix with a bad length is refused", raw: "203.0.113.0/40", want: nil, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			raw := tc.raw

			// when
			prefixes, err := parseAllowlist(raw)

			// then
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			got := make([]string, 0, len(prefixes))
			for _, prefix := range prefixes {
				got = append(got, prefix.String())
			}
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func TestAllowlisted(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		ip   string
		want bool
	}{
		{name: "a bare ipv4 address matches itself", raw: "203.0.113.4", ip: "203.0.113.4", want: true},
		{name: "a bare address does not match a neighbour", raw: "203.0.113.4", ip: "203.0.113.5", want: false},
		{name: "a prefix covers its range", raw: "203.0.113.0/24", ip: "203.0.113.99", want: true},
		{name: "a prefix excludes what is outside it", raw: "203.0.113.0/24", ip: "203.0.114.1", want: false},
		{name: "an ipv6 prefix covers the whole subnet", raw: "2600:387:15:4015::/64", ip: "2600:387:15:4015::4", want: true},
		{name: "an ipv4 mapped entry matches the plain address", raw: "::ffff:203.0.113.4", ip: "203.0.113.4", want: true},
		{name: "several entries are all honoured", raw: "10.0.0.1, 203.0.113.0/24", ip: "203.0.113.7", want: true},
		{name: "an empty allowlist matches nothing", raw: "", ip: "203.0.113.4", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			prefixes, err := parseAllowlist(tc.raw)
			require.NoError(t, err)

			// when
			got := allowlisted(prefixes, netip.MustParseAddr(tc.ip))

			// then
			assert.Equal(t, tc.want, got)
		})
	}
}
