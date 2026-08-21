package geo

import "testing"

func TestIsMainlandChina(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "mainland allocation", value: "1.2.4.8", want: true},
		{name: "another mainland allocation", value: "223.255.252.1", want: true},
		{name: "unmapped public address", value: "8.8.8.8", want: false},
		{name: "private address", value: "192.168.1.1", want: false},
		{name: "ipv4 mapped ipv6", value: "::ffff:1.2.4.8", want: true},
		{name: "mainland ipv6 allocation", value: "2001:da8::1", want: true},
		{name: "unmapped ipv6 address", value: "2001:4860:4860::8888", want: false},
		{name: "invalid address", value: "not-an-ip", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsMainlandChina(test.value); got != test.want {
				t.Fatalf("IsMainlandChina(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
