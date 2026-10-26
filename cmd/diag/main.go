// Command diag lists the HID collections SeeAll can see and prints the
// reports a vendor collection sends, which is how the mouse protocol was
// verified and how a new device's frames are read.
//
//	go run ./cmd/diag -list
//	go run ./cmd/diag -capture -seconds 10
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"seeall/internal/hid"
	"seeall/internal/plugins/mouse"
)

func main() {
	list := flag.Bool("list", false, "list the HID collections on this machine")
	capture := flag.Bool("capture", false, "print the reports one collection sends")
	path := flag.String("path", "", "the collection to capture; empty picks the first battery channel")
	seconds := flag.Int("seconds", 10, "how long to capture")
	flag.Parse()

	if !*list && !*capture {
		flag.Usage()
		os.Exit(2)
	}

	if *list {
		all, err := hid.Enumerate(nil)
		if err != nil {
			log.Fatal(err)
		}
		battery := 0
		for _, i := range all {
			mark := " "
			if isBatteryChannel(i) {
				mark = "*"
				battery++
			}
			fmt.Printf("%s %s in=%d out=%d feature=%d %q\n",
				mark, i.Describe(), i.InputReportByteLength, i.OutputReportByteLength,
				i.FeatureReportByteLength, i.Product)
		}
		fmt.Printf("\n共 %d 个 collection，其中 %d 个是电量通道（* 标记）\n", len(all), battery)
	}

	if *capture {
		captureReports(*path, time.Duration(*seconds)*time.Second)
	}
}

func isBatteryChannel(i hid.Interface) bool {
	return i.VendorID == 0x24AE && i.UsagePage == 0xFF00 && i.Usage == 0x0002
}

func captureReports(path string, d time.Duration) {
	var targets []hid.Interface
	if path == "" {
		all, err := hid.Enumerate(isBatteryChannel)
		if err != nil {
			log.Fatal(err)
		}
		if len(all) == 0 {
			log.Fatal("没有找到电量通道；插上接收器再试，或用 -path 指定 collection")
		}
		targets = all
	} else {
		all, err := hid.Enumerate(nil)
		if err != nil {
			log.Fatal(err)
		}
		for _, i := range all {
			if i.Path == path {
				targets = append(targets, i)
				break
			}
		}
		if len(targets) == 0 {
			log.Fatalf("没有这个 collection：%s", path)
		}
	}

	// A mouse broadcasts on one channel only, and a receiver can expose
	// several: listen to every one of them at once, as the app does.
	type line struct {
		at   time.Time
		who  int
		raw  string
		note string
	}
	lines := make(chan line, 64)
	for i, target := range targets {
		reader, err := hid.OpenReader(target)
		if err != nil {
			fmt.Printf("跳过 %s：%v\n", target.Describe(), err)
			continue
		}
		defer reader.Close()
		fmt.Printf("监听 %s（%s）\n", target.Describe(), describeLength(target))
		go func(i int, reader *hid.Reader) {
			for {
				report, err := reader.Read(300 * time.Millisecond)
				if err != nil {
					return
				}
				if report == nil {
					continue
				}
				text := fmt.Sprintf("% X", report)
				note := ""
				if f, ok := mouse.ParseFrame(report); ok {
					note = fmt.Sprintf("  电量 %d%%  DPI %d  %s", f.Battery, f.DPI, f.LinkText())
				}
				lines <- line{at: time.Now(), who: i, raw: text, note: note}
			}
		}(i, reader)
	}

	fmt.Printf("抓包 %s\n", d)
	started := time.Now()
	deadline := time.After(d)
	count := 0
	for {
		select {
		case <-deadline:
			fmt.Printf("\n共 %d 条报文\n", count)
			return
		case l := <-lines:
			count++
			fmt.Printf("[%6.2fs] #%d %s%s\n", time.Since(started).Seconds(), l.who+1, l.raw, l.note)
		}
	}
}

func describeLength(i hid.Interface) string {
	return fmt.Sprintf("in=%d out=%d", i.InputReportByteLength, i.OutputReportByteLength)
}
