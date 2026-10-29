package system

import (
	"context"
	"strings"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/weather"
)

// The sky is somebody else's to know and slow to change, so it is asked about
// far less often than the machine is sampled: the panel leads with it, but a
// temperature that is a quarter of an hour old is still true.
const (
	// weatherRefresh is how often the service is asked again.
	weatherRefresh = 15 * time.Minute
	// weatherTimeout is how long one answer may take. It is short enough that
	// a machine going to sleep with a fetch in flight does not hold the
	// goroutine for long.
	weatherTimeout = 12 * time.Second
	// weatherStale is when the last report is too old to put on the page. It
	// is three refreshes, so that one or two failures in a row still show the
	// weather rather than flicker back to the score.
	weatherStale = 3 * weatherRefresh
)

// autoCity is what an empty city means, in the word the setting's placeholder
// shows it with.
const autoCity = "自动"

// skyState is what the monitor knows about the weather: the last report,
// whether it has one, and why the last question failed.
//
// It is kept apart from the sample rather than in it because the two run on
// different clocks: the machine is sampled every two seconds, the sky every
// quarter of an hour.
type skyState struct {
	mu  sync.Mutex
	say weather.Report
	has bool
	// why is the last failure, so that a machine that is offline writes one
	// line to the log rather than one every quarter of an hour.
	why string
	// wake asks the watcher to look again now, which is what a refresh does.
	wake chan struct{}
	// client is where reports come from. Nil takes the real services; a test
	// points it somewhere of its own.
	client *weather.Client
}

// watchSky asks about the sky once and then every weatherRefresh, on its own
// goroutine: sampling the machine must never wait on somebody else's server.
func (in *instance) watchSky(quit <-chan struct{}) {
	for {
		in.fetchSky()
		select {
		case <-quit:
			return
		case <-time.After(weatherRefresh):
		case <-in.sky.wake:
		}
	}
}

// fetchSky asks the service and keeps what it said. A failure keeps the last
// report — a stale temperature is closer to the truth than none — and says why
// in the log, once per different failure.
func (in *instance) fetchSky() {
	client := in.sky.client
	if client == nil {
		client = weather.New()
	}
	ctx, cancel := context.WithTimeout(context.Background(), weatherTimeout)
	defer cancel()

	// A city the user named wins over the address service, and needs no
	// network guess at all — it is the answer a proxy cannot move. The setting
	// is read on every fetch rather than kept, so that changing it takes
	// effect on the refresh the settings page already asks for.
	//
	// The field's placeholder says 自动, so a user may type it; the geocoder
	// would then look for a city of that name.
	city := strings.TrimSpace(in.env.Settings().Get("city"))
	if city == autoCity || strings.EqualFold(city, "auto") {
		city = ""
	}
	var report weather.Report
	var err error
	if city != "" {
		report, err = client.FetchCity(ctx, city)
	} else {
		report, err = client.Fetch(ctx)
	}

	in.sky.mu.Lock()
	defer in.sky.mu.Unlock()
	if err != nil {
		if in.sky.why != err.Error() {
			in.sky.why = err.Error()
			in.env.Logf("system: 天气拿不到：%v", err)
		}
		return
	}
	in.sky.why = ""
	in.sky.say = report
	in.sky.has = true
}

// refreshSky asks the watcher to look again now, without waiting for the
// quarter of an hour to be over. It never blocks: a refresh while a fetch is
// already in flight needs no second one.
func (in *instance) refreshSky() {
	wake := in.sky.wake
	if wake == nil {
		return
	}
	select {
	case wake <- struct{}{}:
	default:
	}
}

// weather is the sky as a snapshot carries it — the last report, unless it is
// old enough to be a guess. Nil is the header falling back to the score: a
// number that is true beats a temperature that is not.
func (in *instance) weather() *core.Weather {
	in.sky.mu.Lock()
	defer in.sky.mu.Unlock()
	if !in.sky.has || time.Since(in.sky.say.At) > weatherStale {
		return nil
	}
	return &core.Weather{
		TempC: in.sky.say.TempC,
		Text:  in.sky.say.Text(),
		Kind:  in.sky.say.Kind(),
		Place: in.sky.say.Place,
		At:    in.sky.say.At,
	}
}
