package scraper

import (
	"fmt"
	"testing"
	"time"
)

func TestClacl(t *testing.T) {
	start, _ := time.Parse("2006-01-02 15:04:05", "2022-11-29 00:00:00")
	end, _ := time.Parse("2006-01-02 15:04:05", "2023-01-21 00:00:00")
	step := 5 * time.Minute

	d := end.Sub(start).Minutes()
	steps := d / step.Minutes()
	println(fmt.Sprintf("%v", steps))

}

func TestSlices(t *testing.T) {
	names := []int{1, 2, 3, 4, 5}
	for i := 0; i < len(names); i++ {
		n := names[i]
		println(fmt.Sprintf("%v) %v", i, n))
		if n == 3 || n == 5 {
			names = append(names, 100)
		}
	}
}

func TestSplitIntervalsMinutes(t *testing.T) {
	start, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:00:00")
	end, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:30:00")

	inst := Inst{
		From: start,
		To:   end,
	}

	intervals := SplitInterval(inst)
	if !intervals[0].From.Equal(start) {
		t.Error("first sub0interval start is invalid")
	}

	if !intervals[1].To.Equal(end) {
		t.Error("last interval end is invalid")
	}

	midpoint, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:15:00")
	if !intervals[0].To.Equal(intervals[1].From) {
		t.Error("the two intervals have a gap")
	}

	if !intervals[0].To.Equal(midpoint) {
		t.Error("the split point is invalid")
	}

}

func TestSplitIntervalsMinuteSeconds(t *testing.T) {
	start, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:00:00")
	midpoint, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:00:30")
	end, _ := time.Parse("2006-01-02 15:04:05", "2023-01-23 17:01:00")

	inst := Inst{
		From: start,
		To:   end,
	}

	intervals := SplitInterval(inst)

	if !intervals[0].From.Equal(start) {
		t.Error("first sub0interval start is invalid")
	}

	if !intervals[1].To.Equal(end) {
		t.Error("last interval end is invalid")
	}

	if !intervals[0].To.Equal(intervals[1].From) {
		t.Error("the two intervals have a gap")
	}

	if !intervals[0].To.Equal(midpoint) {
		t.Errorf("the split point is invalid: %s", intervals[0].To.String())
	}

}
