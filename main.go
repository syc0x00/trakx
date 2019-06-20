package main

import (
	"flag"
	"fmt"
	"net/http"
	"runtime"
	"syscall"
	"time"

	"github.com/Syc0x00/Trakx/tracker"
)

const trackerBase = "http://nibba.trade:1337"
const netdataBase = "https://nibba.trade/netdata"

const indexHTML = `
<head>
	<title>Trakx</title>
</head>
<style>
body {
	background-color: black;
	font-family: arial;
	color: #9b9ea3;
	font-size: 25px;
	text-align: center;
	align-items: center;
	display: flex;
	justify-content: center;
}
</style>
<div>
	<p>Trakx is an open p2p tracker. Feel free to use it :)</p>
	<p>Add <span style="background-color: #1dc135; color: black;">` + trackerBase + `/announce</span></p>
	<embed src="` + netdataBase + `/api/v1/badge.svg?chart=go_expvar_Trakx.hits_sec&alarm=trakx_hits&refresh=auto" type="image/svg+xml" height="20"/>
	<embed src="` + netdataBase + `/api/v1/badge.svg?chart=go_expvar_Trakx.hits_sec&alarm=trakx_hits_5min&refresh=auto" type="image/svg+xml" height="20"/>
	<embed src="` + netdataBase + `/api/v1/badge.svg?chart=go_expvar_Trakx.hits_sec&alarm=trakx_hits_1hour&refresh=auto" type="image/svg+xml" height="20"/>
	<embed src="` + netdataBase + `/api/v1/badge.svg?chart=go_expvar_Trakx.errors_per_second&alarm=trakx_errors&refresh=auto" type="image/svg+xml" height="20"/>
	<p>Discord: <3#1527 / Email: tracker@nibba.trade</p>
	<a href='/dmca'>DMCA?</a>
</div>
`

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, indexHTML)
}

func dmca(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://www.youtube.com/watch?v=BwSts2s4ba4", http.StatusMovedPermanently)
}

func scrape(w http.ResponseWriter, r *http.Request) {
	resp := "I'm a teapot\n             ;,'\n     _o_    ;:;'\n ,-.'---`.__ ;\n((j`=====',-'\n `-\\     /\n    `-=-'     This tracker doesn't support /scrape"

	w.WriteHeader(http.StatusTeapot)
	fmt.Fprintf(w, resp)
}

func main() {
	// Get flags
	prodFlag := flag.Bool("x", false, "Production mode")
	portFlag := flag.String("p", "1337", "HTTP port to serve")
	flag.Parse()

	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		panic(err)
	}

	if runtime.GOOS == "darwin" {
		limit.Cur = 24576
	} else {
		limit.Cur = limit.Max
	}

	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		panic(err)
	} else {
		fmt.Printf("Set limit to %v\n", limit.Cur)
	}

	err := tracker.Init(*prodFlag)
	if err != nil {
		panic(err)
	}

	go tracker.Cleaner()
	go tracker.Expvar()

	// Handlers
	trackerMux := http.NewServeMux()
	trackerMux.HandleFunc("/", index)
	trackerMux.HandleFunc("/dmca", dmca)
	trackerMux.HandleFunc("/scrape", scrape)
	trackerMux.HandleFunc("/announce", tracker.AnnounceHandle)

	// Server
	server := http.Server{
		Addr:         ":" + *portFlag,
		Handler:      trackerMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Serve
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
