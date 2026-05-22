package config

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"

	"github.com/ian-kent/envconf"
	"github.com/mailhog/MailHog-Server/monkey"
	"github.com/mailhog/data"
	"github.com/mailhog/storage"
)

// DefaultConfig is the default config
func DefaultConfig() *Config {
	return &Config{
		SMTPBindAddr:        "0.0.0.0:1025",
		APIBindAddr:         "0.0.0.0:8025",
		Hostname:            "mailhog.example",
		MongoURI:            "127.0.0.1:27017",
		MongoDb:             "mailhog",
		MongoColl:           "messages",
		MaildirPath:         "",
		StorageType:         "memory",
		CORSOrigin:          "",
		WebPath:             "",
		MaintenanceInterval: "1h",
		MessageChan:         make(chan *data.Message),
		OutgoingSMTP:        make(map[string]*OutgoingSMTP),
	}
}

// Config is the config, kind of
type Config struct {
	SMTPBindAddr               string
	APIBindAddr                string
	Hostname                   string
	MongoURI                   string
	MongoDb                    string
	MongoColl                  string
	StorageType                string
	CORSOrigin                 string
	MaildirPath                string
	InviteJim                  bool
	MaintenanceEnabled         bool
	MaintenanceInterval        string
	MaintenanceDeleteOlderThan string
	MaintenanceMaxMaildirSize  string
	MaintenanceMaxMessages     int
	MaintenanceMinFreeSpace    string
	MaintenancePolicy          storage.MaintenancePolicy
	Storage                    storage.Storage
	MessageChan                chan *data.Message
	Assets                     func(asset string) ([]byte, error)
	Monkey                     monkey.ChaosMonkey
	OutgoingSMTPFile           string
	OutgoingSMTP               map[string]*OutgoingSMTP
	WebPath                    string
}

// OutgoingSMTP is an outgoing SMTP server config
type OutgoingSMTP struct {
	Name      string
	Save      bool
	Email     string
	Host      string
	Port      string
	Username  string
	Password  string
	Mechanism string
}

var cfg = DefaultConfig()

// Jim is a monkey
var Jim = &monkey.Jim{}

// Configure configures stuff
func Configure() *Config {
	maintenancePolicy, err := parseMaintenancePolicy(cfg)
	if err != nil {
		log.Fatal(err)
	}
	cfg.MaintenancePolicy = maintenancePolicy

	switch cfg.StorageType {
	case "memory":
		log.Println("Using in-memory storage")
		cfg.Storage = storage.CreateInMemory()
	case "mongodb":
		log.Println("Using MongoDB message storage")
		s := storage.CreateMongoDB(cfg.MongoURI, cfg.MongoDb, cfg.MongoColl)
		if s == nil {
			log.Println("MongoDB storage unavailable, reverting to in-memory storage")
			cfg.Storage = storage.CreateInMemory()
		} else {
			log.Println("Connected to MongoDB")
			cfg.Storage = s
		}
	case "maildir":
		log.Println("Using maildir message storage")
		s := storage.CreateMaildir(cfg.MaildirPath, cfg.MaintenancePolicy)
		cfg.Storage = s
	default:
		log.Fatalf("Invalid storage type %s", cfg.StorageType)
	}

	Jim.Configure(func(message string, args ...interface{}) {
		log.Printf(message, args...)
	})
	if cfg.InviteJim {
		cfg.Monkey = Jim
	}

	if len(cfg.OutgoingSMTPFile) > 0 {
		b, err := ioutil.ReadFile(cfg.OutgoingSMTPFile)
		if err != nil {
			log.Fatal(err)
		}
		var o map[string]*OutgoingSMTP
		err = json.Unmarshal(b, &o)
		if err != nil {
			log.Fatal(err)
		}
		cfg.OutgoingSMTP = o
	}

	return cfg
}

// RegisterFlags registers flags
func RegisterFlags() {
	flag.StringVar(&cfg.SMTPBindAddr, "smtp-bind-addr", envconf.FromEnvP("MH_SMTP_BIND_ADDR", "0.0.0.0:1025").(string), "SMTP bind interface and port, e.g. 0.0.0.0:1025 or just :1025")
	flag.StringVar(&cfg.APIBindAddr, "api-bind-addr", envconf.FromEnvP("MH_API_BIND_ADDR", "0.0.0.0:8025").(string), "HTTP bind interface and port for API, e.g. 0.0.0.0:8025 or just :8025")
	flag.StringVar(&cfg.Hostname, "hostname", envconf.FromEnvP("MH_HOSTNAME", "mailhog.example").(string), "Hostname for EHLO/HELO response, e.g. mailhog.example")
	flag.StringVar(&cfg.StorageType, "storage", envconf.FromEnvP("MH_STORAGE", "memory").(string), "Message storage: 'memory' (default), 'mongodb' or 'maildir'")
	flag.StringVar(&cfg.MongoURI, "mongo-uri", envconf.FromEnvP("MH_MONGO_URI", "127.0.0.1:27017").(string), "MongoDB URI, e.g. 127.0.0.1:27017")
	flag.StringVar(&cfg.MongoDb, "mongo-db", envconf.FromEnvP("MH_MONGO_DB", "mailhog").(string), "MongoDB database, e.g. mailhog")
	flag.StringVar(&cfg.MongoColl, "mongo-coll", envconf.FromEnvP("MH_MONGO_COLLECTION", "messages").(string), "MongoDB collection, e.g. messages")
	flag.StringVar(&cfg.CORSOrigin, "cors-origin", envconf.FromEnvP("MH_CORS_ORIGIN", "").(string), "CORS Access-Control-Allow-Origin header for API endpoints")
	flag.StringVar(&cfg.MaildirPath, "maildir-path", envconf.FromEnvP("MH_MAILDIR_PATH", "").(string), "Maildir path (if storage type is 'maildir')")
	flag.BoolVar(&cfg.MaintenanceEnabled, "maintenance-enabled", envconf.FromEnvP("MH_MAINTENANCE_ENABLED", false).(bool), "Enable maildir maintenance cleanup and storage guards")
	flag.StringVar(&cfg.MaintenanceInterval, "maintenance-interval", envconf.FromEnvP("MH_MAINTENANCE_INTERVAL", "1h").(string), "Maildir maintenance interval, e.g. 15m or 1h")
	flag.StringVar(&cfg.MaintenanceDeleteOlderThan, "maintenance-delete-older-than", envconf.FromEnvP("MH_MAINTENANCE_DELETE_OLDER_THAN", "").(string), "Delete maildir messages older than a duration, e.g. 24h, 1d, or 1w")
	flag.StringVar(&cfg.MaintenanceMaxMaildirSize, "maintenance-max-maildir-size", envconf.FromEnvP("MH_MAINTENANCE_MAX_MAILDIR_SIZE", "").(string), "Keep maildir message files under this size, e.g. 4Gi or 5GB")
	flag.IntVar(&cfg.MaintenanceMaxMessages, "maintenance-max-messages", envconf.FromEnvP("MH_MAINTENANCE_MAX_MESSAGES", 0).(int), "Keep no more than this many maildir messages")
	flag.StringVar(&cfg.MaintenanceMinFreeSpace, "maintenance-min-free-space", envconf.FromEnvP("MH_MAINTENANCE_MIN_FREE_SPACE", "").(string), "Keep at least this much filesystem free space, e.g. 1Gi")
	flag.BoolVar(&cfg.InviteJim, "invite-jim", envconf.FromEnvP("MH_INVITE_JIM", false).(bool), "Decide whether to invite Jim (beware, he causes trouble)")
	flag.StringVar(&cfg.OutgoingSMTPFile, "outgoing-smtp", envconf.FromEnvP("MH_OUTGOING_SMTP", "").(string), "JSON file containing outgoing SMTP servers")
	Jim.RegisterFlags()
}
