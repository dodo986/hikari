package engine

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	reDownloadSize = regexp.MustCompile(`Download size: ([\d.]+) MiB`)
	reDownloaded   = regexp.MustCompile(`Downloaded: (\S+) MiB`)
	reETA          = regexp.MustCompile(`ETA: (\d\d:\d\d:\d\d)`)
	reDownSpeed    = regexp.MustCompile(`Download\t- (\S+) MiB`)
	reDiskSpeed    = regexp.MustCompile(`Disk\t- (\S+) MiB`)
	rePercent      = regexp.MustCompile(`Progress: (\d+\.\d+) `)
)

type Frame struct {
	Percent         *float64
	Bytes           string
	ETA             string
	DownSpeed       *float64
	DiskSpeed       *float64
	DownloadSizeMiB *float64
}

type Parser struct {
	frame    Frame
	totalMiB *float64
}

func (p *Parser) SetTotalMiB(total float64) { p.totalMiB = &total }

func (p *Parser) Parse(line string) (Frame, bool) {
	if downloaded := reDownloaded.FindStringSubmatch(line); downloaded != nil {
		p.frame.Bytes = downloaded[1] + "MB"
	}
	if v := number(reDownloadSize, line); v != nil {
		p.frame.DownloadSizeMiB = v
	}
	if m := reETA.FindStringSubmatch(line); m != nil {
		p.frame.ETA = m[1]
	}
	if v := number(reDownSpeed, line); v != nil {
		p.frame.DownSpeed = v
	}
	if v := number(reDiskSpeed, line); v != nil {
		p.frame.DiskSpeed = v
	}
	if v := number(rePercent, line); v != nil {
		p.frame.Percent = v
	}

	if reDownloaded.FindStringSubmatch(line) == nil {
		return Frame{}, false
	}

	// legendary prints no percentage; derive it from the downloaded bytes.
	if p.frame.Percent == nil && p.totalMiB != nil && *p.totalMiB > 0 {
		if done, err := strconv.ParseFloat(strings.TrimSuffix(p.frame.Bytes, "MB"), 64); err == nil {
			pct := math.Min(100, done/(*p.totalMiB)*100)
			p.frame.Percent = &pct
		}
	}
	out := p.frame
	p.frame = Frame{DownloadSizeMiB: out.DownloadSizeMiB}
	return out, true
}

func number(re *regexp.Regexp, line string) *float64 {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return nil
	}
	return &v
}
