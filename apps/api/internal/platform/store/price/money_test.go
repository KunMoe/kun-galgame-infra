package price

import "testing"

func TestFormatMinor(t *testing.T) {
	if got := FormatMinor(968000); got != "9680.00" {
		t.Errorf("FormatMinor(968000) = %q", got)
	}
	if got := FormatMinor(1157); got != "11.57" {
		t.Errorf("FormatMinor(1157) = %q", got)
	}
	if got := FormatMinor(5); got != "0.05" {
		t.Errorf("FormatMinor(5) = %q", got)
	}
}

func TestMinor(t *testing.T) {
	if got := Minor(11.566188); got != 1157 {
		t.Errorf("Minor(11.566188) = %d", got)
	}
	if got := Minor(0.005); got != 1 {
		t.Errorf("Minor(0.005) = %d", got)
	}
}
