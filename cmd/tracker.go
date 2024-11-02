package tracker

import (
	"expvar"
	"fmt"
	gohttp "net/http"
	"time"

	"github.com/crimist/trakx/bencoding"
	"github.com/crimist/trakx/config"
	"github.com/crimist/trakx/pools"
	"github.com/crimist/trakx/stats"
	"github.com/crimist/trakx/storage"
	"github.com/crimist/trakx/tracker/http"
	"github.com/crimist/trakx/tracker/udp"
	"go.uber.org/zap"

	// import database types so init is called
	_ "github.com/crimist/trakx/storage/inmemory"
)

// Run initializes and runs the tracker with the requested configuration settings.
func Run(conf *config.Configuration) {
	var udptracker *udp.Tracker
	var httptracker http.Tracker
	var err error

	zap.L().Info("Loaded configuration, starting trakx...")

	warnings := conf.Validate()
	if warnings&config.WarningUDPValidation != 0 {
		zap.L().Warn("Configuration warning [UDP.ConnDB.Validate]: UDP connection validation is disabled. Do not expose this service to untrusted networks; it could be abused in UDP based amplification attacks.")
	}
	if warnings&config.WarningPeerExpiry != 0 {
		zap.L().Warn("Configuration warning [conf.Announce]: Peer expiry time < announce interval. Peers will expire from the database between announces")
	}

	peerdb, err := storage.Open()
	if err != nil {
		zap.L().Fatal("Failed to initialize storage", zap.Error(err))
	} else {
		zap.L().Info("Initialized storage")
	}

	pools.Initialize(int(conf.Numwant.Limit))

	// TODO: put trackers in a slice of type `Tracker` (iface) and then pass it down to the signal handler which can then call Shutdown() on all of them
	go signalHandler(peerdb, udptracker, &httptracker)

	if conf.Debug.Pprof != 0 {
		go servePprof(conf.Debug.Pprof)
	}

	if conf.HTTP.Mode == config.TrackerModeEnabled {
		zap.L().Info("HTTP tracker enabled", zap.Int("port", conf.HTTP.Port), zap.String("ip", conf.HTTP.IP))

		httptracker.Init(peerdb)
		go func() {
			if err := httptracker.Serve(); err != nil {
				zap.L().Fatal("Failed to serve HTTP tracker", zap.Error(err))
			}
		}()
	} else if conf.HTTP.Mode == config.TrackerModeInfo {
		// serve basic html server
		cache, err := config.GenerateEmbeddedCache()
		if err != nil {
			zap.L().Fatal("failed to generate embedded cache", zap.Error(err))
		}

		// create big interval for announce response to reduce load
		d := bencoding.NewDictionary()
		d.Int64("interval", 86400) // 1 day
		announceResponse := d.GetBytes()

		expvarHandler := expvar.Handler()

		mux := gohttp.NewServeMux()
		mux.HandleFunc("/heartbeat", func(w gohttp.ResponseWriter, r *gohttp.Request) {})
		mux.HandleFunc("/stats", func(w gohttp.ResponseWriter, r *gohttp.Request) {
			expvarHandler.ServeHTTP(w, r)
		})
		mux.HandleFunc("/scrape", func(w gohttp.ResponseWriter, r *gohttp.Request) {})
		mux.HandleFunc("/announce", func(w gohttp.ResponseWriter, r *gohttp.Request) {
			w.Write(announceResponse)
		})

		for filepath, data := range cache {
			dataBytes := []byte(data)
			mux.HandleFunc(filepath, func(w gohttp.ResponseWriter, r *gohttp.Request) {
				w.Write(dataBytes)
			})
		}

		server := gohttp.Server{
			Addr:         fmt.Sprintf(":%d", conf.HTTP.Port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 7 * time.Second,
			IdleTimeout:  0,
		}
		server.SetKeepAlivesEnabled(false)

		zap.L().Info("Running HTTP info server", zap.Int("port", conf.HTTP.Port))
		go func() {
			if err := server.ListenAndServe(); err != nil {
				zap.L().Error("Failed to start HTTP server", zap.Error(err))
			}
		}()
	}

	// UDP tracker
	if conf.UDP.Enabled {
		zap.L().Info("UDP tracker enabled", zap.Int("port", conf.UDP.Port), zap.String("ip", conf.UDP.IP))
		udptracker = udp.NewTracker(peerdb)

		go func() {
			if err := udptracker.Serve(); err != nil {
				zap.L().Fatal("Failed to serve UDP tracker", zap.Error(err))
			}
		}()
	}

	if conf.ExpvarInterval > 0 {
		stats.Publish(peerdb, func() int64 {
			return int64(udptracker.Connections())
		})
	} else {
		zap.L().Debug("Finished Run() no expvar - blocking forever")
		select {}
	}
}
