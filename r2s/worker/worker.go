package worker

import (
	"fmt"
	"github.com/Ahton89/r2s_v2/r2s/redis"
	"github.com/caarlos0/env"
	"github.com/elliotchance/sshtunnel"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type workerStruct struct {
	workersCount  int
	wg            *sync.WaitGroup
	config        config
	redisProdData map[string][]redisProdDataStruct
	sshSocket     ssh.AuthMethod
}

type redisProdDataStruct struct {
	key   string
	value string
}

type config struct {
	RedisProductionHost string   `env:"REDIS_PRODUCTION_HOST" envDefault:"127.0.0.1"`
	RedisProductionPort int      `env:"REDIS_PRODUCTION_PORT" envDefault:"6379"`
	RedisProductionDb   int      `env:"REDIS_PRODUCTION_DB" envDefault:"0"`
	Recipients          []string `env:"RECIPIENTS,required" envSeparator:","`
	Hashes              []string `env:"HASHES,required" envSeparator:","`
	RecipientRedisDbNum int      `env:"RECIPIENT_REDIS_DB_NUM" envDefault:"0"`
	RecipientRedisPort  int      `env:"RECIPIENT_REDIS_PORT" envDefault:"6379"`
	SshUsername         string   `env:"SSH_USERNAME,required"`
	RecipientDomain     string   `env:"RECIPIENT_DOMAIN,required"`
	Debug               bool     `env:"DEBUG" envDefault:"false"`
}

type WorkerInterface interface {
	Run()
}

func New() WorkerInterface {
	return &workerStruct{
		wg:            new(sync.WaitGroup),
		redisProdData: make(map[string][]redisProdDataStruct, 0),
	}
}

func (w *workerStruct) Run() {
	if err := env.Parse(&w.config); err != nil {
		log.WithFields(log.Fields{"error": err}).Fatal("config init")
	}
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err != nil {
		log.Fatal("Can not connect to SSH_AUTH_SOCK")
	} else {
		w.sshSocket = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	if w.config.Debug {
		log.SetLevel(log.DebugLevel)
	}
	log.WithFields(log.Fields{"function": "Run()"}).Debug("run fetchProdData()")
	w.fetchProdData()
	for _, host := range w.config.Recipients {
		log.WithFields(log.Fields{"recipient": strings.TrimSpace(host)}).Debug("run cloning worker")
		go w.cloningWorker(strings.TrimSpace(host))
		w.wg.Add(1)
	}
	w.wg.Wait()
}

func (w *workerStruct) fetchProdData() {
	log.WithFields(log.Fields{"function": "fetchProdData()"}).Debug("init new redis connection")
	prodRedis := redis.New()
	log.WithFields(log.Fields{"function": "fetchProdData()"}).Debug("connect to redis")
	err := prodRedis.Connect(fmt.Sprintf("%s:%d",
		w.config.RedisProductionHost,
		w.config.RedisProductionPort),
		w.config.RedisProductionDb)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Fatal("production redis connection")
	}
	defer prodRedis.Close()
	log.Info("fetching init data from production redis...")
	for _, hash := range w.config.Hashes {
		log.WithFields(log.Fields{"function": "fetchProdData()", "hash": hash}).Debug("fetching...")
		hashesKeys, err := prodRedis.GetHashKeys(hash)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "hash": hash}).Fatal("fetch keys for hash")
		}
		log.WithFields(log.Fields{"function": "fetchProdData()"}).Debug("add fetched data to array")
		for _, hashKey := range hashesKeys {
			w.redisProdData[hash] = append(w.redisProdData[hash], redisProdDataStruct{
				key:   hashKey,
				value: prodRedis.GetHashValues(hash, hashKey),
			})
		}
	}
	log.Info("all production hashes fetched")
}

func (w *workerStruct) cloningWorker(name string) {
	var err error
	var recipientName = fmt.Sprintf("%s.%s", name, w.config.RecipientDomain)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName}).Debug("starting worker")
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName}).Debug("making ssh tunnel")
	tunnel := sshtunnel.NewSSHTunnel(
		fmt.Sprintf("%s@%s", w.config.SshUsername, recipientName),
		w.sshSocket,
		fmt.Sprintf("127.0.0.1:%d", w.config.RecipientRedisPort),
	)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName}).Debug("starting ssh tunnel")
	go tunnel.Start()
	time.Sleep(100 * time.Millisecond)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
		"port":      tunnel.Local.Port}).Debug("ssh tunnel started")
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName}).Debug("connecting to redis from ssh runnel")
	recipientRedis := redis.New()
	err = recipientRedis.Connect(fmt.Sprintf("127.0.0.1:%d",
		tunnel.Local.Port),
		w.config.RecipientRedisDbNum)
	defer recipientRedis.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
			"host":  recipientName,
			"port":  tunnel.Local.Port}).Error("connecting to redis via ssh tunnel")
		w.wg.Done()
	} else {
		log.Infof("cloning data to %s...", recipientName)
		for hash, hashData := range w.redisProdData {
			log.WithFields(log.Fields{
				"function":  "cloningWorker()",
				"recipient": recipientName,
				"hash":      hash}).Debug("cloning data")
			for _, data := range hashData {
				err = recipientRedis.SetHash(hash, data.key, data.value)
				if err != nil {
					log.Error(err.Error())
					w.wg.Done()
				}
			}
		}
		log.Infof("cloning data to %s success", recipientName)
		w.wg.Done()
	}
}
