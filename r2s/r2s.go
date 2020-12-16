package r2s

import (
	"github.com/onetwotrip/r2s_v2/r2s/worker"
	log "github.com/sirupsen/logrus"
)

func Run(version string) {
	log.Infof("redis clone utility v.%s starting...", version)
	ottWorker := worker.New()
	ottWorker.Run()
}
