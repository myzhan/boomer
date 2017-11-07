package boomer

import "testing"

func TestRound(t *testing.T) {

	if int(round(float64(147.5002), .5, -1)) != 150 {
		t.Error("147.5002 should be rounded to 150")
	}

	if int(round(float64(3432.5002), .5, -2)) != 3400 {
		t.Error("3432.5002 should be rounded to 3400")
	}

	roundOne := round(float64(58760.5002), .5, -3)
	roundTwo := round(float64(58960.6003), .5, -3)
	if roundOne != roundTwo {
		t.Error("round(58760.5002) should be equal to round(58960.6003)")
	}

	roundOne = round(float64(58360.5002), .5, -3)
	roundTwo = round(float64(58460.6003), .5, -3)
	if roundOne != roundTwo {
		t.Error("round(58360.5002) should be equal to round(58460.6003)")
	}
}
