package common

import "testing"

func TestHumanSize(t *testing.T) {
	cases := []struct {
		bytes float64
		want  string
	}{
		{0, "0"},
		{512, "512 B"},
		{1024, "1.00 KiB"},
		{7939842485, "7.39 GiB"},
		{12884901888, "12.00 GiB"},
		{1099511627776, "1.00 TiB"},
	}
	for _, c := range cases {
		if got := HumanSize(c.bytes); got != c.want {
			t.Errorf("HumanSize(%v): want %q, got %q", c.bytes, c.want, got)
		}
	}
}
