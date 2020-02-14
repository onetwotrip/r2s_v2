package worker

import (
	"errors"
	"fmt"
	"github.com/onetwotrip/r2s_v2/r2s/redis"
	"github.com/caarlos0/env"
	"github.com/elliotchance/sshtunnel"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"net"
	"os"
	"strconv"
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
	errorsCount   int
	errorHosts    []fieldsStruct
	mt            *sync.Mutex
}

type redisProdDataStruct struct {
	key   string
	value string
}

type recipient struct {
	host string
	port int
	db   int
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
	SshAuthSocket       string   `env:"SSH_AUTH_SOCK,required"`
	RecipientDomain     string   `env:"RECIPIENT_DOMAIN,required"`
	Debug               bool     `env:"DEBUG" envDefault:"false"`
	ExitIfError         bool     `env:"EXIT_IF_ERROR" envDefault:"false"`
	BuildUrl            string   `env:"BUILD_URL" envDefault:"https://example.com"`
	SlackHookUrl        string   `env:"SLACK_HOOK_URL,required"`
}

type WorkerInterface interface {
	Run()
}

func New() WorkerInterface {
	return &workerStruct{
		wg:            new(sync.WaitGroup),
		redisProdData: make(map[string][]redisProdDataStruct, 0),
		mt:            new(sync.Mutex),
	}
}

func (w *workerStruct) errorCounterIncrement() {
	w.mt.Lock()
	w.errorsCount += 1
	w.mt.Unlock()
}

func (w *workerStruct) parseRecipient(r string) (*recipient, error) {
	data := &recipient{}
	var port, db int
	var err error
	r = strings.TrimSpace(r)
	hostTempData := strings.Split(r, ":")
	switch len(hostTempData) {
	case 1:
		data.host = r
		data.port = w.config.RecipientRedisPort
		data.db = w.config.RecipientRedisDbNum
		return data, nil
	case 2:
		data.host = hostTempData[0]
		port, err = strconv.Atoi(hostTempData[1])
		if err != nil {
			return data, err
		}
		data.port = port
		data.db = w.config.RecipientRedisDbNum
		return data, nil
	case 3:
		data.host = hostTempData[0]
		port, err = strconv.Atoi(hostTempData[1])
		if err != nil {
			return data, err
		}
		data.port = port
		db, err = strconv.Atoi(hostTempData[2])
		if err != nil {
			return data, err
		}
		data.db = db
		return data, nil
	default:
		return data, errors.New("undefined format, required host:port:db")
	}
}

func (w *workerStruct) Run() {
	var err error
	if err = env.Parse(&w.config); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("config init")
	}
	if w.config.Debug {
		log.SetLevel(log.DebugLevel)
	}
	log.WithFields(log.Fields{
		"function":      "Run()",
		"SSH_AUTH_SOCK": w.config.SshAuthSocket,
	}).Debug("get SSH_AUTH_SOCK")
	if sshAgent, err := net.Dial("unix", w.config.SshAuthSocket); err != nil {
		log.Fatal("Can't connect to SSH_AUTH_SOCK")
	} else {
		w.sshSocket = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	log.WithFields(log.Fields{
		"function": "Run()",
	}).Debug("run fetchProdData()")
	w.fetchProdData()
	for _, host := range w.config.Recipients {
		log.WithFields(log.Fields{
			"function":  "Run()",
			"recipient": strings.TrimSpace(host),
		}).Debug("parsing host to recipient struct")
		hostParsed, err := w.parseRecipient(host)
		if err != nil {
			log.WithFields(log.Fields{
				"recipient": strings.TrimSpace(host),
				"error":     err.Error(),
			}).Error("parsing host")
			continue
		} else {
			go w.cloningWorker(hostParsed.host, hostParsed.port, hostParsed.db)
			log.WithFields(log.Fields{
				"recipient": hostParsed.host,
				"port":      hostParsed.port,
				"db":        hostParsed.db,
			}).Debug("run cloning worker")
			w.wg.Add(1)
		}
	}
	w.wg.Wait()
	if len(w.errorHosts) > 0 {
		log.WithFields(log.Fields{
			"function":   "Run()",
			"errorHosts": w.errorHosts,
		}).Debug("slack notify")
		err = w.sendSlackMessage()
		if err != nil {
			log.WithFields(log.Fields{
				"function": "Run()",
				"error":    err.Error(),
			}).Error("slack notify")
			w.errorCounterIncrement()
		}
	}
	if w.config.ExitIfError {
		if w.errorsCount > 0 {
			os.Exit(1)
		}
	}
}

func (w *workerStruct) fetchProdData() {
	log.WithFields(log.Fields{
		"function": "fetchProdData()",
	}).Debug("init new redis connection")
	prodRedis := redis.New()
	log.WithFields(log.Fields{
		"function": "fetchProdData()",
	}).Debug("connect to redis")
	err := prodRedis.Connect(fmt.Sprintf("%s:%d",
		w.config.RedisProductionHost,
		w.config.RedisProductionPort),
		w.config.RedisProductionDb)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("production redis connection")
	}
	defer prodRedis.Close()
	log.Info("fetching init data from production redis...")
	for _, hash := range w.config.Hashes {
		log.WithFields(log.Fields{
			"function": "fetchProdData()",
			"hash":     hash,
		}).Debug("fetching...")
		hashesKeys, err := prodRedis.GetHashKeys(hash)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"hash":  hash,
			}).Fatal("fetch keys for hash")
		}
		log.WithFields(log.Fields{
			"function": "fetchProdData()",
		}).Debug("add fetched data to array")
		for _, hashKey := range hashesKeys {
			w.redisProdData[hash] = append(w.redisProdData[hash], redisProdDataStruct{
				key:   hashKey,
				value: prodRedis.GetHashValues(hash, hashKey),
			})
		}
		log.WithFields(log.Fields{
			"function": "fetchProdData()",
			"hash":     hash,
			"keys":     len(w.redisProdData[hash]),
		}).Debug("fetched keys info")
	}
	log.Info("all production hashes fetched")
}

func (w *workerStruct) cloningWorker(name string, port, db int) {
	var workerError = false
	var err error
	var recipientName = fmt.Sprintf("%s.%s", name, w.config.RecipientDomain)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
	}).Debug("starting worker")
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
	}).Debug("making ssh tunnel")
	tunnel := sshtunnel.NewSSHTunnel(
		fmt.Sprintf("%s@%s", w.config.SshUsername, recipientName),
		w.sshSocket,
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
	}).Debug("starting ssh tunnel")
	go tunnel.Start()
	time.Sleep(100 * time.Millisecond)
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
		"port":      tunnel.Local.Port,
	}).Debug("ssh tunnel started")
	log.WithFields(log.Fields{
		"function":  "cloningWorker()",
		"recipient": recipientName,
	}).Debug("connecting to redis from ssh runnel")
	recipientRedis := redis.New()
	err = recipientRedis.Connect(fmt.Sprintf("127.0.0.1:%d",
		tunnel.Local.Port),
		db)
	defer recipientRedis.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
			"host":  recipientName,
			"port":  tunnel.Local.Port,
		}).Debugf("connection to %s redis via ssh tunnel failed...", recipientName)
		workerError = true
	} else {
		log.Infof("cloning data to %s...", recipientName)
		for hash, hashData := range w.redisProdData {
			log.WithFields(log.Fields{
				"function":  "cloningWorker()",
				"recipient": recipientName,
				"hash":      hash,
			}).Debug("cloning data")
			for _, data := range hashData {
				err = recipientRedis.SetHash(hash, data.key, data.value)
				if err != nil {
					log.WithFields(log.Fields{
						"function":  "cloningWorker()",
						"recipient": recipientName,
						"hash":      hash,
						"error":     err.Error(),
					}).Debug("cloning data")
					workerError = true
					break
				}
			}
		}
	}
	if workerError {
		log.Errorf("cloning data to %s failed", recipientName)
		w.errorCounterIncrement()
		w.appendErrorHost(name)
	} else {
		log.Infof("cloning data to %s success", recipientName)
	}
	w.wg.Done()
}
