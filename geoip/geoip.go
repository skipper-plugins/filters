package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang"
	"github.com/zalando/skipper/filters"
	snet "github.com/zalando/skipper/net"
)

type geoipSpec struct {
	db *maxminddb.Reader
}

func InitFilter(opts []string) (filters.Spec, error) {
	var db string
	for _, o := range opts {
		switch {
		case strings.HasPrefix(o, "db="):
			db = o[3:]
		}
	}
	if db == "" {
		return nil, fmt.Errorf("missing db= parameter for geoip plugin")
	}
	reader, err := maxminddb.Open(db)
	if err != nil {
		return nil, fmt.Errorf("failed to open db %s: %s", db, err)
	}
	return geoipSpec{db: reader}, nil
}

func (s geoipSpec) Name() string {
	return "geoip"
}

func (s geoipSpec) CreateFilter(config []interface{}) (filters.Filter, error) {
	var fromLast bool
	header := "X-GeoIP-Country"
	var err error
	for _, c := range config {
		if s, ok := c.(string); ok {
			switch {
			case strings.HasPrefix(s, "from_last="):
				fromLast, err = strconv.ParseBool(s[10:])
				if err != nil {
					return nil, filters.ErrInvalidFilterParameters
				}
			case strings.HasPrefix(s, "header="):
				header = s[7:]
			}
		}
	}
	return &geoipFilter{db: s.db, fromLast: fromLast, header: header}, nil
}

type geoipFilter struct {
	db       *maxminddb.Reader
	fromLast bool
	header   string
}

type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func (f *geoipFilter) Request(c filters.FilterContext) {
	var src net.IP
	if f.fromLast {
		src = snet.RemoteHostFromLast(c.Request())
	} else {
		src = snet.RemoteHost(c.Request())
	}

	record := countryRecord{}
	err := f.db.Lookup(src, &record)
	if err != nil {
		fmt.Printf("geoip(): failed to lookup %s: %s", src, err)
		c.Request().Header.Set(f.header, "unknown")
		return
	}
	if record.Country.ISOCode == "" {
		record.Country.ISOCode = "unknown"
	}
	c.Request().Header.Set(f.header, record.Country.ISOCode)
}

func (f *geoipFilter) Response(c filters.FilterContext) {
	/*
		// for debugging, set the X-GeoIP-Country on the response...
		var src net.IP
		if f.fromLast {
			src = snet.RemoteHostFromLast(c.Request())
		} else {
			src = snet.RemoteHost(c.Request())
		}
		record := countryRecord{}
		err := f.db.Lookup(src, &record)
		if err != nil {
			fmt.Printf("geoip(): failed to lookup %s: %s", src, err)
			c.Response().Header.Set("X-GeoIP-Country", "unknown")
			return
		}
		if record.Country.ISOCode == "" {
			record.Country.ISOCode = "unknown"
		}
		c.Response().Header.Set("X-GeoIP-Country", record.Country.ISOCode)
	*/
}
