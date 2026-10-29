package system

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"seeall/internal/weather"
)

// aWeatherService answers the way the two services do, with the geocoder
// echoing the name it was asked about: a test can then see which city the
// monitor actually asked for. The address services fail, so a fetch that
// reaches them cannot pass.
func aWeatherService(t *testing.T) *weather.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geo":
			fmt.Fprintf(w, `{"results":[{"name":%q,"latitude":29.5,"longitude":106.5}]}`,
				r.URL.Query().Get("name"))
		case "/sky":
			w.Write([]byte(`{"current":{"temperature_2m":17.6,"weather_code":3}}`))
		default:
			http.Error(w, "the address service is not needed here", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return &weather.Client{
		IP: srv.URL + "/ip", Who: srv.URL + "/who",
		Geo: srv.URL + "/geo", Sky: srv.URL + "/sky",
	}
}

// TestTheCitySettingWinsOverTheAddressService checks the setting is what the
// monitor asks about: a machine behind a proxy that leaves through another
// country still gets the weather of the city its user named.
func TestTheCitySettingWinsOverTheAddressService(t *testing.T) {
	in := newTestInstance(t)
	in.sky.client = aWeatherService(t)
	in.env.Settings().Set("city", "重庆")
	in.fetchSky()

	sky := in.weather()
	if sky == nil {
		t.Fatalf("a named city produced no weather; log: %v", in.sky.why)
	}
	if sky.Place != "重庆" || sky.TempC != 17.6 {
		t.Errorf("the page says %q %v°C, want 重庆 17.6°C", sky.Place, sky.TempC)
	}
}

// TestTheCitySettingTakesEffectOnTheNextFetch checks the setting is read on
// every fetch rather than kept: saving it in the settings page asks for a
// refresh, and a refresh has to be enough — no restart of the monitor.
func TestTheCitySettingTakesEffectOnTheNextFetch(t *testing.T) {
	in := newTestInstance(t)
	in.sky.client = aWeatherService(t)

	in.env.Settings().Set("city", "重庆")
	in.fetchSky()
	if sky := in.weather(); sky == nil || sky.Place != "重庆" {
		t.Fatalf("the first city did not reach the page: %+v", sky)
	}

	in.env.Settings().Set("city", "成都")
	in.fetchSky()
	sky := in.weather()
	if sky == nil {
		t.Fatal("the second city produced no weather")
	}
	if sky.Place != "成都" {
		t.Errorf("the page still says %q after the setting changed, want 成都", sky.Place)
	}
}

// TestAnEmptyCitySettingIsTheAddressService checks the setting's default is
// "ask the address service": clearing it — or typing the word its placeholder
// shows — goes back to guessing, rather than leaving the monitor asking a
// geocoder for a city called 自动.
func TestAnEmptyCitySettingIsTheAddressService(t *testing.T) {
	for _, value := range []string{"", "  ", autoCity, "AUTO"} {
		in := newTestInstance(t)
		in.sky.client = aWeatherService(t)
		in.env.Settings().Set("city", value)
		in.fetchSky()

		if in.weather() != nil {
			t.Errorf("city %q with no address service still showed weather", value)
		}
		if in.sky.why == "" {
			t.Errorf("city %q failed without saying why", value)
		}
	}
}

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
