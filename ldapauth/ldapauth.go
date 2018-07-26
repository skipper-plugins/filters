package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zalando/skipper/filters"
	"gopkg.in/ldap.v2"
)

type ldapAuthSpec struct {
	BindUser     string
	BindPassword string
	BaseDN       string
	Scope        int
	Filter       string
	DNTemplate   string
	URL          *url.URL
	Insecure     bool
}

type ldapAuthFilter struct {
	Realm        string
	BindUser     string
	BindPassword string
	BaseDN       string
	Scope        int
	Filter       string
	DNTemplate   string
	URL          *url.URL
	Insecure     bool
}

func InitFilter(opts []string) (filters.Spec, error) {
	spec := &ldapAuthSpec{}
	var err error
	for _, o := range opts {
		switch {
		case strings.HasPrefix(o, "uri="):
			spec.URL, err = url.Parse(o[4:])
			if err != nil {
				return nil, fmt.Errorf("failed to parse uri= parameter for ldap_auth plugin")
			}
		case strings.HasPrefix(o, "base="):
			spec.BaseDN = o[5:]
		case strings.HasPrefix(o, "user="):
			spec.BindUser = o[5:]
		case strings.HasPrefix(o, "pass="):
			spec.BindPassword = o[5:]
		case strings.HasPrefix(o, "scope="):
			switch o[6:] {
			case "sub", "subtree":
				spec.Scope = ldap.ScopeWholeSubtree
			case "one", "single":
				spec.Scope = ldap.ScopeSingleLevel
			case "base":
				spec.Scope = ldap.ScopeBaseObject
			default:
				return nil, fmt.Errorf("invalid scope value `%s`", o[6:])
			}
		case strings.HasPrefix(o, "filter="):
			spec.Filter = o[7:]
		case strings.HasPrefix(o, "template="):
			spec.DNTemplate = o[9:]
		case strings.HasPrefix(o, "insecure="):
			spec.Insecure, err = strconv.ParseBool(o[9:])
			if err != nil {
				return nil, fmt.Errorf("failed to parse insecure= parameter for ldap_auth plugin")
			}
		}
	}

	if spec.URL == nil {
		return nil, fmt.Errorf("missing uri= parameter for ldap_auth plugin")
	}
	host, port, err := net.SplitHostPort(spec.URL.Host)
	if err != nil || port == "" {
		switch spec.URL.Scheme {
		case "ldaps":
			port = "636"
		case "ldap":
			port = "389"
		default:
			return nil, fmt.Errorf("Unknown scheme '%s'", spec.URL.Scheme)
		}
		spec.URL.Host += host + ":" + port
	}

	if spec.DNTemplate == "" {
		if spec.BindUser == "" || spec.BindPassword == "" || spec.BaseDN == "" || spec.Filter == "" {
			return nil, fmt.Errorf("Missing values in config, check `filter=`, `base=`, `user=` and/or `pass=`")
		}
	}
	return spec, nil
}

func (s *ldapAuthSpec) Name() string {
	return "ldapAuth"
}

func (s *ldapAuthSpec) CreateFilter(config []interface{}) (filters.Filter, error) {
	realm := ""
	for _, c := range config {
		if s, ok := c.(string); ok {
			realm = s
		}
	}
	return &ldapAuthFilter{
		Realm:        realm,
		BaseDN:       s.BaseDN,
		BindUser:     s.BindUser,
		BindPassword: s.BindPassword,
		DNTemplate:   s.DNTemplate,
		Scope:        s.Scope,
		URL:          s.URL,
		Insecure:     s.Insecure,
	}, nil
}

func (f *ldapAuthFilter) unauthorized(c filters.FilterContext, msg string) {
	header := http.Header{}
	header.Set("WWW-Authenticate", f.Realm)
	bodyBuf := bytes.NewBuffer(nil)
	bodyBuf.Write([]byte(msg))
	c.Serve(&http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     header,
		Body:       ioutil.NopCloser(bodyBuf),
	})
}

func (f *ldapAuthFilter) connect() (conn ldap.Client, err error) {
	maxTries := 3
	for i := 0; i < maxTries; i++ {
		if f.URL.Scheme == "ldaps" {
			var tlsCfg *tls.Config
			if f.Insecure {
				tlsCfg = &tls.Config{InsecureSkipVerify: true}
			}
			conn, err = ldap.DialTLS("tcp", f.URL.Host, tlsCfg)
		} else {
			conn, err = ldap.Dial("tcp", f.URL.Host)
		}
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return conn, nil
	}
	return nil, fmt.Errorf("failed to connect to '%s': %s", f.URL, err)
}

func (f *ldapAuthFilter) Request(c filters.FilterContext) {
	auth := c.Request().Header.Get("Authorization")
	if auth == "" {
		f.unauthorized(c, "missing Authorization: header")
		return
	}
	parts := strings.SplitN(strings.TrimSpace(auth), " ", 1)
	if len(parts) == 1 {
		f.unauthorized(c, "invalid Authorization: header")
		return
	}
	if strings.ToLower(parts[0]) != "basic" {
		f.unauthorized(c, "invalid Authorization: type")
		return
	}

	auth = parts[1]
	data, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		f.unauthorized(c, "invalid base64 encoded value")
		return
	}

	parts = strings.SplitN(string(data), ":", 1)
	if len(parts) == 1 {
		f.unauthorized(c, "missing password in Authorization: header")
		return
	}

	conn, err := f.connect()
	if err != nil {
		f.unauthorized(c, "invalid username or password")
		return
	}
	defer conn.Close()

	user := parts[0]
	pass := parts[1]

	var bindDN, bindPass string
	if f.DNTemplate != "" {
		bindDN = fmt.Sprintf(f.DNTemplate, user)
		bindPass = pass
	} else {
		bindDN = f.BindUser
		bindPass = f.BindPassword
	}

	if err := conn.Bind(bindDN, bindPass); err != nil {
		f.unauthorized(c, "invalid username or password")
		return
	}

	if f.DNTemplate != "" {
		c.Request().Header.Del("Authorization")
		c.Request().Header.Set("X-Authenticated-User", user)
		return
	}

	res, err := conn.Search(&ldap.SearchRequest{
		BaseDN:     f.BaseDN,
		Filter:     fmt.Sprintf(f.Filter, ldap.EscapeFilter(user)),
		Attributes: []string{"dn"},
		Scope:      f.Scope,
	})
	if err != nil {
		f.unauthorized(c, "invalid username or password")
		return
	}
	if len(res.Entries) != 1 {
		f.unauthorized(c, "invalid username or password")
		return
	}

	if err := conn.Bind(res.Entries[0].DN, pass); err != nil {
		f.unauthorized(c, "invalid username or password")
		return
	}

	c.Request().Header.Del("Authorization")
	c.Request().Header.Set("X-Authenticated-User", user)
}

func (f *ldapAuthFilter) Response(c filters.FilterContext) {}
