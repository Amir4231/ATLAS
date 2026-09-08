package geo

import "testing"

func TestHaversineKnownDistance(t *testing.T) {
	d := HaversineM(3.1390, 101.6869, 3.1391, 101.6870)
	if d < 10 || d > 25 {
		t.Fatalf("want ~15m got %v", d)
	}
	if HaversineM(3.1, 101.6, 3.1, 101.6) != 0 {
		t.Fatal("same point must be 0")
	}
}
