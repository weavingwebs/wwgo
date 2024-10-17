package wwgo

import (
	"github.com/beevik/ntp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"sync"
	"time"
)

type NtpTime struct {
	log            zerolog.Logger
	ntpServer      string
	mut            sync.RWMutex
	timeOffset     time.Duration
	lastNtpRefresh time.Time
}

func NewNtpTime(log zerolog.Logger, ntpServer string) (*NtpTime, error) {
	if ntpServer == "" {
		return nil, errors.Errorf("ntpServer is empty")
	}

	return &NtpTime{
		log:       log,
		ntpServer: ntpServer,
	}, nil
}

func (nt *NtpTime) GetTimeAndOffset() (time.Time, time.Duration) {
	nt.mut.Lock()
	defer nt.mut.Unlock()

	// Do not refresh if it's been less than an hour.
	now := time.Now()
	if now.Sub(nt.lastNtpRefresh) < time.Hour {
		return now.Add(nt.timeOffset), nt.timeOffset
	}

	// Get latest NTP time.
	resp, err := ntp.Query(nt.ntpServer)
	if err != nil {
		panic(errors.Wrapf(err, "Failed to get NTP time from %s", nt.ntpServer))
	}
	nt.log.Debug().Dur("offset", resp.ClockOffset).Msgf("Got NTP time from %s", nt.ntpServer)
	nt.timeOffset = resp.ClockOffset
	nt.lastNtpRefresh = now
	return resp.Time, resp.ClockOffset
}

func (nt *NtpTime) GetTime() time.Time {
	t, _ := nt.GetTimeAndOffset()
	return t
}

func (nt *NtpTime) GetTimeOffset() time.Duration {
	_, offset := nt.GetTimeAndOffset()
	return offset
}
