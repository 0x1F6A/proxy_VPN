// mmdb reader 适配层 — 真实实现走 oschwald/maxminddb-golang，但单测可替换。
package geoip

import (
	"fmt"
	"net"
	"os"

	"github.com/oschwald/maxminddb-golang"
)

type mmdbReader interface {
	lookupCountry(ip net.IP) string
	close() error
}

type realReader struct {
	r *maxminddb.Reader
}

type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func (rr *realReader) lookupCountry(ip net.IP) string {
	var rec countryRecord
	if err := rr.r.Lookup(ip, &rec); err != nil {
		return ""
	}
	return rec.Country.ISOCode
}

func (rr *realReader) close() error { return rr.r.Close() }

func openReader(path string) (mmdbReader, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("geoip mmdb %q: %w", path, err)
	}
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	return &realReader{r: r}, nil
}
