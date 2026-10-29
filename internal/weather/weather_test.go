package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// sky is a stand-in for the four services: one server, four paths, so that a
// test can say what each of them answers and which ones fail.
type sky struct {
	server *httptest.Server
	// mu guards asked, which the server writes on its own goroutines.
	mu sync.Mutex
	// asked is every path the client asked for, in order.
	asked []string
	// fail names the paths that answer 500.
	fail map[string]bool
}

// newSky starts the stand-in. The caller closes it.
func newSky(t *testing.T, fail ...string) *sky {
	t.Helper()
	s := &sky{fail: map[string]bool{}}
	for _, path := range fail {
		s.fail[path] = true
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, r.URL.Path)
		failing := s.fail[r.URL.Path]
		s.mu.Unlock()
		if failing {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		switch r.URL.Path {
		case "/ip":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("当前 IP：123.147.244.125  来自于：中国 重庆 重庆  联通\n"))
		case "/who":
			w.Write([]byte(`{"success":true,"city":"Hong Kong","latitude":22.31,"longitude":114.17}`))
		case "/geo":
			w.Write([]byte(`{"results":[{"latitude":29.56026,"longitude":106.55771}]}`))
		case "/sky":
			w.Write([]byte(`{"current":{"temperature_2m":17.6,"weather_code":3}}`))
		default:
			http.NotFound(w, r)
		}
	})
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// pathsAsked is what the client has asked for so far.
func (s *sky) pathsAsked() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

// client points a client at the stand-in.
func (s *sky) client() *Client {
	return &Client{
		IP:  s.server.URL + "/ip",
		Who: s.server.URL + "/who",
		Geo: s.server.URL + "/geo",
		Sky: s.server.URL + "/sky",
		Now: func() time.Time { return time.Date(2026, 10, 29, 13, 0, 0, 0, time.UTC) },
	}
}

// TestFetchGivesTheSkyOverTheCity checks the whole chain: the city in the
// address service's words, its coordinates, and the temperature and code the
// forecast service answers with.
func TestFetchGivesTheSkyOverTheCity(t *testing.T) {
	s := newSky(t)
	report, err := s.client().Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Place != "重庆" {
		t.Errorf("the report is for %q, want 重庆", report.Place)
	}
	if report.TempC != 17.6 || report.Code != 3 {
		t.Errorf("the report is %v°C code %d, want 17.6°C code 3",
			report.TempC, report.Code)
	}
	if got := report.Text(); got != "阴" {
		t.Errorf("an overcast sky reads %q, want 阴", got)
	}
	if got := report.Kind(); got != "cloud" {
		t.Errorf("an overcast sky is drawn as %q, want cloud", got)
	}
	if want := time.Date(2026, 10, 29, 13, 0, 0, 0, time.UTC); !report.At.Equal(want) {
		t.Errorf("the report is stamped %v, want the client's clock %v", report.At, want)
	}
}

// TestTheLocationServicesAreAskedInOrder checks the fallback: a machine whose
// address service does not answer still gets the weather, from the service
// that answers with the city and the coordinates at once.
func TestTheLocationServicesAreAskedInOrder(t *testing.T) {
	s := newSky(t, "/ip")
	report, err := s.client().Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Place != "Hong Kong" {
		t.Errorf("the fallback located %q, want Hong Kong", report.Place)
	}
	for _, path := range s.pathsAsked() {
		if path == "/geo" {
			t.Errorf("the city's coordinates were asked for after no city was named: %v",
				s.pathsAsked())
		}
	}
}

// TestNoWeatherIsAnError checks a report is all or nothing: a forecast service
// that fails is an error, not a report of 0°C for the right city — which the
// header would draw as though it were true.
func TestNoWeatherIsAnError(t *testing.T) {
	s := newSky(t, "/sky")
	report, err := s.client().Fetch(context.Background())
	if err == nil {
		t.Fatalf("a failed forecast read %+v, want an error", report)
	}
	if report.Place != "" || report.TempC != 0 {
		t.Errorf("a failed fetch reported %+v, want nothing", report)
	}
}

// TestCityOfReadsTheCity checks the words the address service answers with: the
// city is the third word, because a province that is a city repeats itself in
// the second one.
func TestCityOfReadsTheCity(t *testing.T) {
	for _, c := range []struct {
		body string
		city string
	}{
		{"当前 IP：1.2.3.4  来自于：中国 重庆 重庆  联通", "重庆"},
		{"当前 IP：1.2.3.4  来自于：中国 北京 北京  电信", "北京"},
		{"当前 IP：1.2.3.4  来自于：中国 广东 深圳  电信", "深圳"},
		{"当前 IP：1.2.3.4  来自于：中国 广东 深圳\n", "深圳"},
	} {
		got, err := cityOf(c.body)
		if err != nil {
			t.Errorf("cityOf(%q) failed: %v", c.body, err)
			continue
		}
		if got != c.city {
			t.Errorf("cityOf(%q) = %q, want %q", c.body, got, c.city)
		}
	}
	if _, err := cityOf("当前 IP：1.2.3.4\n"); err == nil {
		t.Error("an answer that names no place was read as a city")
	}
}

// TestTheCodesHaveWords checks every code group the header can show: the words
// beside the temperature, and the shape drawn before it.
func TestTheCodesHaveWords(t *testing.T) {
	for _, c := range []struct {
		code int
		text string
		kind string
	}{
		{0, "晴", "sun"},
		{1, "少云", "partly"},
		{2, "多云", "partly"},
		{3, "阴", "cloud"},
		{45, "雾", "fog"},
		{48, "雾", "fog"},
		{53, "毛毛雨", "rain"},
		{63, "小雨", "rain"},
		{73, "雪", "snow"},
		{81, "阵雨", "rain"},
		{85, "阵雪", "snow"},
		{95, "雷雨", "thunder"},
		{99, "雷雨", "thunder"},
	} {
		r := Report{Code: c.code}
		if got := r.Text(); got != c.text {
			t.Errorf("code %d reads %q, want %q", c.code, got, c.text)
		}
		if got := r.Kind(); got != c.kind {
			t.Errorf("code %d is drawn as %q, want %q", c.code, got, c.kind)
		}
	}
	// A code from the future still gets a temperature and a shape; only the
	// words are left out, instead of showing a code number to a person.
	r := Report{Code: 4000, TempC: 12}
	if got := r.Text(); got != "" {
		t.Errorf("an unknown code reads %q, want no words", got)
	}
	if got := r.Kind(); got != "cloud" {
		t.Errorf("an unknown code is drawn as %q, want the least it can say", got)
	}
}

// TestTheAnswersAreCapped checks the client does not trust a service with its
// memory: an answer longer than the cap is read up to the cap and no further.
func TestTheAnswersAreCapped(t *testing.T) {
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", maxBody*2)))
	}))
	defer long.Close()

	body, err := (&Client{}).get(context.Background(), long.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > maxBody {
		t.Errorf("read %d bytes of an answer, want at most %d", len(body), maxBody)
	}
}
