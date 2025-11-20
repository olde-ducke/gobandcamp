package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/term"
)

type pipe struct{ stdin, stderr *os.File }

func (p *pipe) Read(b []byte) (int, error)  { return p.stdin.Read(b) }
func (p *pipe) Write(b []byte) (int, error) { return p.stderr.Write(b) }

func readInput(password bool, format string, a ...any) (line string, err error) {
	fd := int(os.Stdin.Fd())

	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() {
		// NOTE: if this fails, terminal will be left in raw mode
		if err = term.Restore(fd, state); err != nil {
			log.Default().Println("failed to restore terminal:", err)
		}
	}()

	fmt.Fprintf(os.Stderr, format, a...)
	t := term.NewTerminal(&pipe{os.Stdin, os.Stderr}, "")

	if password {
		line, err = t.ReadPassword("")
	} else {
		line, err = t.ReadLine()
	}
	if err != nil {
		fmt.Fprint(os.Stderr, "\r\n")
		return "", err
	}

	return line, nil
}

// NOTE: this is mostly copy of httproxy function
func parseProxyURL(proxy string) (*url.URL, error) {
	u, err := url.Parse(proxy)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if u, err := url.Parse("http://" + proxy); err == nil {
			return u, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return u, nil
}

func setProxyCredentials(proxy string) (*url.URL, error) {
	u, err := parseProxyURL(proxy)
	if err != nil {
		return nil, err
	}

	username, err := readInput(false, "enter username for %q: ", u.Redacted())
	if err != nil {
		return nil, err
	}

	if username == "" && u.User != nil && u.User.Username() != "" {
		username = u.User.Username()
	}

	password, err := readInput(true, "enter password for %q: ", u.Redacted())
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(username, password)

	return u, nil
}

func mergeProxyEnvFlag(prompt bool, env string, flag string) (*url.URL, error) {
	if env == "" && flag == "" {
		return &url.URL{}, nil
	}

	if prompt {
		if flag != "" {
			return setProxyCredentials(flag)
		}
		return setProxyCredentials(env)
	}

	// NOTE: validate only flag value, don't throw error for env value,
	// by default http.Client ignores invalid values from environment.
	if flag != "" {
		return parseProxyURL(flag)
	}

	u, err := parseProxyURL(env)
	if err != nil {
		u = &url.URL{}
	}

	return u, nil
}

type options struct {
	cpuProfile             string
	memProfile             string
	debug                  bool
	httpProxy              string
	httpsProxy             string
	noProxy                string
	promptProxyCredentials bool
	sampleRate             int

	logFile *os.File
}

func readOptions() (int, *options) {
	opt := options{
		sampleRate: 44100,
	}

	var help, version bool

	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.SetOutput(os.Stderr)

	f.StringVar(&opt.cpuProfile, "cpu-profile", opt.cpuProfile,
		"write cpu profile to a `file`")
	f.BoolVar(&opt.debug, "debug", opt.debug,
		"write debug output to 'dump.log' file")
	f.BoolVar(&opt.debug, "d", opt.debug,
		"write debug output to 'dump.log' file")
	f.BoolVar(&help, "help", help, "show this message and exit")
	f.BoolVar(&help, "h", help, "show this message and exit")
	f.StringVar(&opt.httpProxy, "http-proxy", opt.httpProxy,
		"URL of the HTTP proxy server")
	f.StringVar(&opt.httpsProxy, "https-proxy", opt.httpProxy,
		"URL of the HTTPS proxy server")
	f.StringVar(&opt.memProfile, "mem-profile", opt.memProfile,
		"write memory profile to a `file`")
	f.StringVar(&opt.noProxy, "no-proxy", opt.noProxy,
		"comma-separated list of hosts that should be excluded from proxying")
	f.BoolVar(&version, "version", version, "show version and exit")
	f.BoolVar(&version, "v", version, "show version and exit")
	f.BoolVar(&opt.promptProxyCredentials, "w", opt.promptProxyCredentials,
		"prompt proxy username and password")
	f.IntVar(&opt.sampleRate, "sample-rate", opt.sampleRate,
		"sample rate of player")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return 2, nil
	}

	if help {
		f.Usage()
		return 0, nil
	}

	if version {
		info, ok := debug.ReadBuildInfo()
		if ok {
			fmt.Println(info)
			return 0, nil
		}

		fmt.Fprintln(os.Stderr,
			"build info is not available without module support enabled")
		return 1, nil
	}

	if args := f.Args(); len(args) > 0 {
		fmt.Fprintf(os.Stderr,
			"unexpected arguments, first one: %q\n", args[0])
		return 2, nil
	}

	cfg := httpproxy.FromEnvironment()

	var parseErr *url.Error

	u, err := mergeProxyEnvFlag(opt.promptProxyCredentials,
		cfg.HTTPProxy, opt.httpProxy)
	if errors.As(err, &parseErr) {
		fmt.Fprintln(os.Stderr, "invalid HTTP proxy URL:", err)
		return 2, nil
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read user input:", err)
		return 1, nil
	}

	cfg.HTTPProxy = u.String()
	opt.httpProxy = u.Redacted()

	u, err = mergeProxyEnvFlag(opt.promptProxyCredentials,
		cfg.HTTPSProxy, opt.httpsProxy)
	if errors.As(err, &parseErr) {
		fmt.Fprintln(os.Stderr, "invalid HTTPS proxy URL:", err)
		return 2, nil
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read user input:", err)
		return 1, nil
	}

	cfg.HTTPSProxy = u.String()
	opt.httpsProxy = u.Redacted()

	if opt.noProxy != "" {
		cfg.NoProxy = opt.noProxy
	} else if cfg.NoProxy != "" {
		// for logging
		opt.noProxy = cfg.NoProxy
	}

	proxyFunc := cfg.ProxyFunc()
	http.DefaultTransport.(*http.Transport).Proxy =
		func(req *http.Request) (*url.URL, error) {
			return proxyFunc(req.URL)
		}

	if opt.sampleRate <= 0 {
		// FIXME: limit upper bound?
		fmt.Fprintln(os.Stderr, "invalid sample rate value:", opt.sampleRate)
		return 2, nil
	}

	// NOTE: open file as a last step, so it could be properly closed in main
	if opt.debug {
		f, err := os.Create("dump.log")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1, nil
		}

		opt.logFile = f
		log.Default().SetOutput(io.MultiWriter(os.Stderr, f))
	}

	return -1, &opt
}
