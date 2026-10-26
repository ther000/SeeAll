// Package server watches a service elsewhere: an HTTP endpoint, its status
// code, how long it took and how long it has been failing.
package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"seeall/internal/core"
	"seeall/internal/plugin"
)

// Manifest describes the plugin to the app.
func Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:              "server",
		Name:            "服务器",
		Summary:         "HTTP 服务的状态码、延迟与证书到期时间",
		Kind:            plugin.KindNetwork,
		Icon:            "server",
		Multiple:        true,
		DefaultInstance: "example.com",
		Interval:        15 * time.Second,
		Settings: []plugin.Setting{
			{Key: "url", Label: "地址", Type: plugin.SettingText, Default: "https://example.com",
				Placeholder: "https://example.com/health",
				Description: "要监视的地址，每次请求一次"},
			{Key: "expect", Label: "期望状态码", Type: plugin.SettingNumber, Default: "200",
				Description: "留空表示 2xx–3xx 都算正常"},
			{Key: "timeout", Label: "超时（秒）", Type: plugin.SettingNumber, Default: "10"},
			{Key: "cert", Label: "检查证书到期", Type: plugin.SettingBool, Default: "true"},
			{Key: "notify", Label: "故障提醒", Type: plugin.SettingBool, Default: "true"},
		},
	}
}

// Plugin is the server monitor.
type Plugin struct{}

// Manifest implements plugin.Plugin.
func (Plugin) Manifest() plugin.Manifest { return Manifest() }

// New implements plugin.Plugin.
func (Plugin) New(env *plugin.Env) plugin.Instance {
	return &instance{
		env:      env,
		client:   &http.Client{},
		history:  core.NewRing(60),
		notified: map[string]bool{},
	}
}

type instance struct {
	env    *plugin.Env
	client *http.Client

	mu          sync.Mutex
	history     *core.Ring
	failures    int
	lastErr     string
	lastStatus  int
	lastLatency time.Duration
	lastOK      time.Time
	up          bool
	notifyOn    bool
	notified    map[string]bool

	started bool
	quit    chan struct{}
	wg      sync.WaitGroup
}

// Start implements plugin.Instance.
func (in *instance) Start() {
	in.mu.Lock()
	if in.started {
		in.mu.Unlock()
		return
	}
	in.started = true
	in.quit = make(chan struct{})
	quit := in.quit
	in.mu.Unlock()

	in.wg.Add(1)
	go func() {
		defer in.wg.Done()
		in.check()
		ticker := time.NewTicker(in.interval())
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				in.check()
			}
		}
	}()
}

// Stop implements plugin.Instance.
func (in *instance) Stop() {
	in.mu.Lock()
	quit := in.quit
	in.started = false
	in.quit = nil
	in.mu.Unlock()
	if quit == nil {
		return
	}
	close(quit)
	in.wg.Wait()
}

// Poll implements plugin.Instance.
func (in *instance) Poll() { go in.check() }

func (in *instance) interval() time.Duration {
	seconds := in.env.Settings().Int("interval", 15)
	if seconds < 5 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}

// check makes one request and records what came back.
func (in *instance) check() {
	settings := in.env.Settings()
	raw := strings.TrimSpace(settings.Get("url"))
	if raw == "" {
		in.env.Publish(core.Snapshot{
			At:       time.Now(),
			Status:   core.StatusOffline,
			Title:    "未设置地址",
			Subtitle: "在设置里填要监视的地址",
		})
		return
	}
	target, err := url.Parse(raw)
	if err != nil || target.Scheme == "" {
		target, err = url.Parse("https://" + raw)
	}
	if err != nil {
		in.env.Publish(core.Snapshot{At: time.Now(), Status: core.StatusError, Title: raw, Subtitle: "地址无效"})
		return
	}

	timeout := time.Duration(settings.Int("timeout", 10)) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	started := time.Now()
	resp, err := client.Get(target.String())
	latency := time.Since(started)
	status := 0
	var certDays int = -1
	if resp != nil {
		status = resp.StatusCode
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 && settings.Bool("cert", true) {
			certDays = int(time.Until(resp.TLS.PeerCertificates[0].NotAfter).Hours() / 24)
		}
	} else if tlsErr, ok := err.(*tls.CertificateVerificationError); ok && tlsErr != nil {
		certDays = -2
	}

	in.mu.Lock()
	in.lastLatency = latency
	in.lastStatus = status
	in.history.Push(float64(latency / time.Millisecond))
	if err != nil {
		in.failures++
		in.lastErr = shorten(err.Error())
		in.up = false
	} else {
		expect := strings.TrimSpace(settings.Get("expect"))
		ok := true
		if expect != "" {
			if want, convErr := strconv.Atoi(expect); convErr == nil {
				ok = status == want
			}
		} else {
			ok = status >= 200 && status < 400
		}
		if ok {
			in.failures = 0
			in.lastErr = ""
			in.lastOK = time.Now()
			in.up = true
		} else {
			in.failures++
			in.lastErr = fmt.Sprintf("状态码 %d", status)
			in.up = false
		}
	}
	failures, lastErr, up := in.failures, in.lastErr, in.up
	lastOK, lastStatus := in.lastOK, in.lastStatus
	history := in.history.Values()
	in.mu.Unlock()

	snapStatus := core.StatusOK
	if !up {
		if failures == 0 {
			snapStatus = core.StatusStarting
		} else {
			snapStatus = core.StatusError
		}
	}

	readings := []core.Reading{}
	if lastStatus > 0 {
		severity := core.SeverityOK
		if lastStatus >= 500 {
			severity = core.SeverityCritical
		} else if lastStatus >= 400 {
			severity = core.SeverityWarn
		}
		readings = append(readings, core.Reading{
			ID: "status", Label: "状态码", Value: float64(lastStatus), Unit: core.UnitNone,
			Severity: severity,
		})
	}
	readings = append(readings, core.Reading{
		ID: "latency", Label: "延迟", Value: float64(latency.Milliseconds()), Unit: core.UnitMillis,
		Severity: latencySeverity(latency), Detail: trend(history),
	})
	if len(history) > 0 {
		var sum float64
		for _, v := range history {
			sum += v
		}
		readings = append(readings, core.Reading{
			ID: "avg", Label: "平均延迟", Value: sum / float64(len(history)), Unit: core.UnitMillis,
			Severity: core.SeverityNone, Detail: fmt.Sprintf("%d 次采样", len(history)),
		})
	}
	if certDays >= 0 {
		severity := core.SeverityOK
		switch {
		case certDays <= 7:
			severity = core.SeverityCritical
		case certDays <= 21:
			severity = core.SeverityWarn
		}
		readings = append(readings, core.Reading{
			ID: "cert", Label: "证书", Value: float64(certDays), Unit: core.UnitDay,
			Severity: severity, Detail: "到期前",
		})
	}
	if failures > 0 {
		readings = append(readings, core.Reading{
			ID: "failures", Label: "连续失败", Value: float64(failures), Unit: core.UnitNone,
			Severity: core.SeverityCritical, Detail: lastErr,
		})
	}
	if !lastOK.IsZero() && up {
		readings = append(readings, core.Reading{
			ID: "uptime", Label: "上次成功", Text: core.FormatDuration(time.Since(lastOK)),
			Severity: core.SeverityNone,
		})
	}

	subtitle := target.Host
	if !up && lastErr != "" {
		subtitle = lastErr
	}
	in.env.Publish(core.Snapshot{
		At:       time.Now(),
		Status:   snapStatus,
		Title:    target.Host,
		Subtitle: subtitle,
		Primary:  "latency",
		Readings: readings,
		Err:      lastErr,
	})
	in.maybeNotify(up, target.Host, lastErr)
}

// latencySeverity colors a request that took too long.
func latencySeverity(d time.Duration) core.Severity {
	switch {
	case d > 3*time.Second:
		return core.SeverityCritical
	case d > 800*time.Millisecond:
		return core.SeverityWarn
	default:
		return core.SeverityOK
	}
}

// trend compares the newest latency with the oldest one kept.
func trend(history []float64) string {
	if len(history) < 4 {
		return ""
	}
	first, last := history[0], history[len(history)-1]
	if first <= 0 {
		return ""
	}
	change := (last - first) / first * 100
	switch {
	case change > 10:
		return fmt.Sprintf("比之前慢 %.0f%%", change)
	case change < -10:
		return fmt.Sprintf("比之前快 %.0f%%", -change)
	default:
		return "延迟平稳"
	}
}

func shorten(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= 60 {
		return s
	}
	return string([]rune(s)[:60]) + "…"
}

// maybeNotify posts one notification when a service goes down and another
// when it comes back.
func (in *instance) maybeNotify(up bool, host, reason string) {
	if !in.env.Settings().Bool("notify", true) {
		return
	}
	in.mu.Lock()
	down := in.notified["down"]
	in.mu.Unlock()
	if !up && !down {
		in.mu.Lock()
		in.notified["down"] = true
		in.mu.Unlock()
		in.env.Notify(host+" 有故障", reason)
		return
	}
	if up && down {
		in.mu.Lock()
		delete(in.notified, "down")
		in.mu.Unlock()
		in.env.Notify(host+" 恢复了", "服务重新正常响应。")
	}
}
