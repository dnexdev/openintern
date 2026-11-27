package yamlite

import (
	"strings"
	"testing"
)

func TestHugeHexStaysPositive(t *testing.T) {
	n, err := Parse("t.yaml", []byte("v: 0xFFFFFFFFFFFFFFFF\n"))
	if err != nil {
		t.Fatal(err)
	}
	v := n.Get("v")
	if v.Kind != Float || v.Float <= 0 || v.Int != 0 {
		t.Fatalf("got kind=%v int=%d float=%g; want a positive float", v.Kind, v.Int, v.Float)
	}
}

func TestOutOfRangeEscapeRejected(t *testing.T) {
	_, err := Parse("t.yaml", []byte("v: \"\\UFFFFFFFF\"\n"))
	if err == nil || !strings.Contains(err.Error(), "bad \\U escape") {
		t.Fatalf("err = %v, want bad \\U escape", err)
	}
}
