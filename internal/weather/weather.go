// Package weather says what the sky over the machine is doing, and over which
// place it is doing it.
//
// Two keyless services answer the question: one says where the machine sits on
// the internet, the other says what the sky is doing there. Neither needs an
// account, and either can fail — a machine that is offline, one whose traffic
// leaves through a proxy in another country, a service having a bad day. A
// caller that gets an error keeps whatever it had and asks again later, which
// is why this package reports a whole Report or nothing at all.
//
// The address service can be skipped: FetchCity asks about a city the user
// named, which is the one answer a proxy cannot move.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The services, as they answer today. Each is overridable on a Client so that
// a test can point it at a server of its own.
const (
	// DefaultIP answers "当前 IP：1.2.3.4  来自于：中国 重庆 重庆  联通". It is
	// a Chinese service and answers a home connection with the home's own
	// city, which is the question the header is really asking.
	DefaultIP = "https://myip.ipip.net"
	// DefaultWho answers the same question in JSON, with the coordinates
	// already in it: the fallback when the one above does not answer.
	DefaultWho = "https://ipwho.is/"
	// DefaultGeo turns a city's name into coordinates.
	DefaultGeo = "https://geocoding-api.open-meteo.com/v1/search"
	// DefaultSky is the forecast service: keyless, and the only one of the
	// four that is asked about the sky itself.
	DefaultSky = "https://api.open-meteo.com/v1/forecast"
)

// maxBody is how much of an answer is read. Every service answers in a few
// hundred bytes; the cap is what keeps a wrong address from filling memory.
const maxBody = 1 << 16

// Report is the sky over one place at one moment.
type Report struct {
	// TempC is the air temperature in degrees Celsius.
	TempC float64
	// Code is the WMO weather code the words and the shape come from.
	Code int
	// Place is the city the report is for: "重庆".
	Place string
	// At is when it was fetched.
	At time.Time
}

// Text names the sky in a word, in the words the dashboard's header writes
// beside the temperature. A code this package does not know has no words: an
// empty Text is a header that shows the temperature alone.
func (r Report) Text() string {
	switch {
	case r.Code == 0:
		return "晴"
	case r.Code == 1:
		return "少云"
	case r.Code == 2:
		return "多云"
	case r.Code == 3:
		return "阴"
	case r.Code == 45, r.Code == 48:
		return "雾"
	case r.Code >= 51 && r.Code <= 57:
		return "毛毛雨"
	case r.Code >= 61 && r.Code <= 67:
		return "小雨"
	case r.Code >= 71 && r.Code <= 77:
		return "雪"
	case r.Code >= 80 && r.Code <= 82:
		return "阵雨"
	case r.Code == 85, r.Code == 86:
		return "阵雪"
	case r.Code >= 95 && r.Code <= 99:
		return "雷雨"
	}
	return ""
}

// Kind is the shape the sky is drawn as: the sun, the sun behind a cloud, a
// covered sky, fog, rain, snow or a storm. An unknown code is drawn as a
// covered sky, which says the least of the seven.
func (r Report) Kind() string {
	switch {
	case r.Code == 0:
		return "sun"
	case r.Code == 1, r.Code == 2:
		return "partly"
	case r.Code == 3:
		return "cloud"
	case r.Code == 45, r.Code == 48:
		return "fog"
	case r.Code >= 71 && r.Code <= 77, r.Code == 85, r.Code == 86:
		return "snow"
	case r.Code >= 95 && r.Code <= 99:
		return "thunder"
	case r.Code >= 51 && r.Code <= 67, r.Code >= 80 && r.Code <= 82:
		return "rain"
	}
	return "cloud"
}

// Client fetches the sky. Its addresses are fields so that a test can point it
// at a server of its own; an empty one takes the service named above.
type Client struct {
	// HTTP is the connection to use. Nil takes a client that gives up after
	// eight seconds, which is short enough that a fetch cannot outlive the
	// refresh it was made for.
	HTTP *http.Client
	// IP, Who, Geo and Sky are the four addresses, in the order they are asked.
	IP, Who, Geo, Sky string
	// Now is the clock. Nil takes the real one.
	Now func() time.Time
}

// New makes a client for the real services.
func New() *Client { return &Client{} }

// Fetch asks where the machine is, then what the sky is doing there. The two
// questions are separate calls, so a failure in either is an error rather than
// a report for a place the machine is not in.
func (c *Client) Fetch(ctx context.Context) (Report, error) {
	where, err := c.locate(ctx)
	if err != nil {
		return Report{}, err
	}
	return c.report(ctx, where)
}

// FetchCity asks the sky over a city the user named, without asking the address
// service where the machine is: a machine whose traffic leaves through a proxy
// in another country would be given that country's weather, and the user's own
// answer is the one no proxy can move.
//
// The name is whatever the geocoder understands — 重庆, Chongqing, 东京 — and
// the report is named the way the geocoder names it, so that "chongqing" comes
// back as 重庆 rather than as what was typed.
func (c *Client) FetchCity(ctx context.Context, name string) (Report, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Report{}, fmt.Errorf("没有说城市")
	}
	where, err := c.coordinates(ctx, name)
	if err != nil {
		return Report{}, err
	}
	return c.report(ctx, where)
}

// report is the sky over a place already known.
func (c *Client) report(ctx context.Context, where place) (Report, error) {
	code, temp, err := c.sky(ctx, where)
	if err != nil {
		return Report{}, err
	}
	return Report{TempC: temp, Code: code, Place: where.city, At: c.now()}, nil
}

// place is where the machine is: the city it is named by and the coordinates
// the sky is asked for.
type place struct {
	city string
	lat  float64
	lon  float64
}

// locate finds the place: the city from the service that answers in words,
// its coordinates from the geocoder, and — when either of those fail — the
// service that answers with both at once.
func (c *Client) locate(ctx context.Context) (place, error) {
	city, cityErr := c.city(ctx)
	if cityErr == nil && city != "" {
		// The geocoder names the place too, and that name is used rather than
		// the address service's words: the manual city and the guessed one
		// then read the same way in the header.
		if where, err := c.coordinates(ctx, city); err == nil {
			return where, nil
		}
	}
	where, err := c.whoIs(ctx)
	if err != nil {
		if cityErr != nil {
			return place{}, cityErr
		}
		return place{}, err
	}
	return where, nil
}

// city is the machine's city, as the address service words it.
func (c *Client) city(ctx context.Context) (string, error) {
	body, err := c.get(ctx, or(c.IP, DefaultIP))
	if err != nil {
		return "", err
	}
	return cityOf(string(body))
}

// cityOf reads the city out of an answer like
// "当前 IP：1.2.3.4  来自于：中国 重庆 重庆  联通".
//
// The answer is the country, the province and then the city, followed by the
// carrier, so the city is the third word. A province that is a city of its own
// — 北京, 重庆 — repeats itself there, which is why the third word is the right
// one rather than the second.
func cityOf(body string) (string, error) {
	const mark = "来自于："
	at := strings.Index(body, mark)
	if at < 0 {
		return "", fmt.Errorf("定位服务没说地点：%q", strings.TrimSpace(firstLine(body)))
	}
	words := strings.Fields(body[at+len(mark):])
	switch {
	case len(words) >= 3:
		return words[2], nil
	case len(words) == 2:
		return words[1], nil
	case len(words) == 1:
		return words[0], nil
	}
	return "", fmt.Errorf("定位服务没说地点：%q", strings.TrimSpace(firstLine(body)))
}

// whoIs is the fallback locator: one answer with the city and its coordinates.
func (c *Client) whoIs(ctx context.Context) (place, error) {
	body, err := c.get(ctx, or(c.Who, DefaultWho))
	if err != nil {
		return place{}, err
	}
	var answer struct {
		Success   bool    `json:"success"`
		City      string  `json:"city"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return place{}, fmt.Errorf("定位服务答非所问：%v", err)
	}
	if !answer.Success || answer.City == "" {
		return place{}, fmt.Errorf("定位服务没说地点")
	}
	return place{city: answer.City, lat: answer.Latitude, lon: answer.Longitude}, nil
}

// coordinates turns a city's name into the coordinates the forecast service
// asks for, and names the place the way the geocoder does: what the user typed
// may be a name only the geocoder can read.
func (c *Client) coordinates(ctx context.Context, city string) (place, error) {
	address := or(c.Geo, DefaultGeo) + "?count=1&language=zh&name=" + url.QueryEscape(city)
	body, err := c.get(ctx, address)
	if err != nil {
		return place{}, err
	}
	var answer struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return place{}, fmt.Errorf("坐标服务答非所问：%v", err)
	}
	if len(answer.Results) == 0 {
		return place{}, fmt.Errorf("找不到 %s 的坐标", city)
	}
	first := answer.Results[0]
	if first.Name == "" {
		first.Name = city
	}
	return place{city: first.Name, lat: first.Latitude, lon: first.Longitude}, nil
}

// sky is the temperature and the weather code at a place.
func (c *Client) sky(ctx context.Context, where place) (code int, temp float64, err error) {
	address := or(c.Sky, DefaultSky) +
		"?current=temperature_2m,weather_code" +
		"&latitude=" + strconv.FormatFloat(where.lat, 'f', 4, 64) +
		"&longitude=" + strconv.FormatFloat(where.lon, 'f', 4, 64)
	body, err := c.get(ctx, address)
	if err != nil {
		return 0, 0, err
	}
	var answer struct {
		Current struct {
			TempC float64 `json:"temperature_2m"`
			Code  int     `json:"weather_code"`
		} `json:"current"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return 0, 0, fmt.Errorf("天气服务答非所问：%v", err)
	}
	return answer.Current.Code, answer.Current.TempC, nil
}

// get reads one answer, and says what went wrong in the words the log wants.
func (c *Client) get(ctx context.Context, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s 答了 %s", address, resp.Status)
	}
	return body, nil
}

// now is the time a report was fetched.
func (c *Client) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}

// or is a field with the real service behind it.
func or(field, real string) string {
	if field != "" {
		return field
	}
	return real
}

// firstLine is the start of an answer, for a message that quotes it.
func firstLine(body string) string {
	if at := strings.IndexByte(body, '\n'); at >= 0 {
		return body[:at]
	}
	if len(body) > 120 {
		return body[:120]
	}
	return body
}
