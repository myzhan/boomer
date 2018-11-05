package boomer

import "testing"

func TestConvertResponseTime(t *testing.T) {
	convertedFloat := convertResponseTime(float64(1.234))
	convertedInt64 := convertResponseTime(int64(2))
	if convertedFloat != 1 {
		t.Error("Failed to convert responseTime from float64")
	}
	if convertedInt64 != 2 {
		t.Error("Failed to convert responseTime from int64")
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("It should panic")
		}
	}()
	// It should panic
	convertResponseTime(1)
}
