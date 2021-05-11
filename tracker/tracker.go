package tracker

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/crimist/trakx/bencoding"
	"github.com/crimist/trakx/tracker/config"
	trakxhttp "github.com/crimist/trakx/tracker/http"
	"github.com/crimist/trakx/tracker/storage"
	"github.com/crimist/trakx/tracker/udp"
	"go.uber.org/zap"

	// import database types so init is called
	_ "github.com/crimist/trakx/tracker/storage/map"
)

// Run runs the tracker
func Run() {
	var udptracker udp.UDPTracker
	var httptracker trakxhttp.HTTPTracker
	var err error

	rand.Seed(time.Now().UnixNano() * time.Now().Unix())

	if !config.Conf.Loaded() {
		config.Logger.Fatal("Config failed to load critical values")
	}

	config.Logger.Info("Loaded configuration, starting trakx...")

	// db
	peerdb, err := storage.Open()
	if err != nil {
		config.Logger.Fatal("Failed to initialize storage", zap.Error(err))
	}

	// init the peerchan with minimum
	storage.PeerChan.Add(config.Conf.PeerChanMin)

	// run signal handler
	go signalHandler(peerdb, &udptracker, &httptracker)

	// init pprof if enabled
	if config.Conf.PprofPort != 0 {
		config.Logger.Info("pprof enabled", zap.Int("port", config.Conf.PprofPort))
		initpprof()
	}

	if config.Conf.Tracker.HTTP.Enabled {
		config.Logger.Info("http tracker enabled", zap.Int("port", config.Conf.Tracker.HTTP.Port))

		httptracker.Init(peerdb)
		go httptracker.Serve()
	} else {
		// serve basic html server with index and dmca pages
		d := bencoding.NewDict()
		d.Int64("interval", 432000) // 5 days
		errResp := []byte(d.Get())

		trackerMux := http.NewServeMux()
		trackerMux.HandleFunc("/", index)
		trackerMux.HandleFunc("/dmca", dmca)
		trackerMux.HandleFunc("/scrape", func(w http.ResponseWriter, r *http.Request) {})
		trackerMux.HandleFunc("/announce", func(w http.ResponseWriter, r *http.Request) {
			w.Write(errResp)
		})

		server := http.Server{
			Addr:         fmt.Sprintf(":%d", config.Conf.Tracker.HTTP.Port),
			Handler:      trackerMux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 7 * time.Second,
			IdleTimeout:  0,
		}
		server.SetKeepAlivesEnabled(false)

		go func() {
			if err := server.ListenAndServe(); err != nil {
				config.Logger.Error("ListenAndServe()", zap.Error(err))
			}
		}()
	}

	// UDP tracker
	if config.Conf.Tracker.UDP.Enabled {
		config.Logger.Info("udp tracker enabled", zap.Int("port", config.Conf.Tracker.UDP.Port))
		udptracker.Init(peerdb)
		go udptracker.Serve()
	}

	publishExpvar(peerdb, &httptracker, &udptracker)
}
