package core

import (
	"testing"
	"time"
)

// TestFormatSpan pins the uptime pill's wording: "2天2小时" is how long the
// machine has been up, while FormatDuration's "2 天前" is when it started.
// The two are one wrong call apart, and the dashboard shows the result.
func TestFormatSpan(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0秒"},
		{9 * time.Second, "9秒"},
		{59 * time.Second, "59秒"},
		{18 * time.Minute, "18分钟"},
		{5*time.Hour + 12*time.Minute, "5小时12分"},
		{49*time.Hour + 59*time.Minute, "2天1小时"},
		{2*24*time.Hour + 2*time.Hour + 30*time.Minute, "2天2小时"},
		{-time.Hour, "0秒"},
	}
	for _, c := range cases {
		if got := FormatSpan(c.d); got != c.want {
			t.Errorf("FormatSpan(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// TestFactText checks that a labeled fact reads as one pill ("已运行 2天2小时")
// and an unlabeled one keeps the value alone, which is what a processor's name
// needs: "CPU Ryzen 7 6800H" would say the label twice, the card above says it.
func TestFactText(t *testing.T) {
	cases := []struct {
		fact Fact
		want string
	}{
		{Fact{Value: "Ryzen 7 6800H"}, "Ryzen 7 6800H"},
		{Fact{Label: "已运行", Value: "2天2小时"}, "已运行 2天2小时"},
	}
	for _, c := range cases {
		if got := c.fact.Text(); got != c.want {
			t.Errorf("Fact%+v.Text() = %q, want %q", c.fact, got, c.want)
		}
	}
}
