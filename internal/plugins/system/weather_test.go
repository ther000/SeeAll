package system

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"seeall/internal/weather"
)

// TestThePageCarriesTheSky checks the weather reaches the snapshot in the words
// and the shape the panel draws, rather than as a code the panel would have to
// know: the panel says what the sky looks like, the plugin says what it is.
func TestThePageCarriesTheSky(t *testing.T) {
	in := newTestInstance(t)
	in.sky.say = weather.Report{
		TempC: 17.6, Code: 3, Place: "重庆", At: time.Now(),
	}
	in.sky.has = true
	in.collect()

	sky := in.env.Snapshot().Weather
	if sky == nil {
		t.Fatal("a fetched sky did not reach the page")
	}
	if sky.TempC != 17.6 {
		t.Errorf("the page says %v°C, want 17.6", sky.TempC)
	}
	if sky.Text != "阴" || sky.Kind != "cloud" {
		t.Errorf("code 3 reached the page as %q / %q, want 阴 / cloud", sky.Text, sky.Kind)
	}
	if sky.Place != "重庆" {
		t.Errorf("the sky is for %q, want 重庆", sky.Place)
	}
}

// TestAnOldSkyIsNotOnThePage checks a report that has not been refreshed for
// long enough is dropped rather than shown: the header then leads with the
// score again, which is a number that is still true.
func TestAnOldSkyIsNotOnThePage(t *testing.T) {
	in := newTestInstance(t)
	in.sky.say = weather.Report{TempC: 17.6, Code: 3, Place: "重庆",
		At: time.Now().Add(-2 * weatherStale)}
	in.sky.has = true
	in.collect()

	if sky := in.env.Snapshot().Weather; sky != nil {
		t.Errorf("a sky fetched %v ago is still on the page: %+v", 2*weatherStale, sky)
	}
}

// TestAFailedFetchKeepsTheLastSky checks the failure path the user actually
// meets — offline, a proxy in another country, a service having a bad day: the
// temperature the panel already has stays, and the reason is written to the log
// once rather than every time the service is asked.
func TestAFailedFetchKeepsTheLastSky(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer down.Close()

	in := newTestInstance(t)
	var lines []string
	in.env.SetLog(func(format string, args ...any) {
		lines = append(lines, format)
	})
	in.sky.client = &weather.Client{
		IP: down.URL + "/ip", Who: down.URL + "/who",
		Geo: down.URL + "/geo", Sky: down.URL + "/sky",
	}
	in.sky.say = weather.Report{TempC: 5, Code: 3, Place: "重庆", At: time.Now()}
	in.sky.has = true

	in.fetchSky()
	in.fetchSky()

	sky := in.weather()
	if sky == nil {
		t.Fatal("a failed fetch dropped the sky the panel already had")
	}
	if sky.TempC != 5 || sky.Text != "阴" {
		t.Errorf("the kept sky is %v°C %q, want 5°C 阴", sky.TempC, sky.Text)
	}
	if len(lines) != 1 {
		t.Fatalf("a lasting failure wrote %d log lines, want 1: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "天气拿不到") {
		t.Errorf("the log line does not say what went wrong: %q", lines[0])
	}
}
