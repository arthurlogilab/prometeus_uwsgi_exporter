package main

import (
	"flag"
    "path"
    "os"
	"io/ioutil"
	"net/http"
    "net"
//	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
    "fmt"
    "runtime"
)

type StatsSoketConf_t struct {
		Domain string               `yaml:"domain"`
		Soket string                `yaml:"soket"`
}

type Config_t struct {
	Port int                        `yaml:"port"`
	SoketDir string                 `yaml:"soket_dir"`
    PIDPath string                  `yaml:"pidfile"`
    LogFilePath string              `yaml:"logfile"`
	StatsSokets []StatsSoketConf_t  `yaml:"stats_sokets"`
}

// LOGGER
var log = logging.MustGetLogger("uwsg_exporter")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05} %{shortfunc}  %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

func SetUpLogger() {
    file, err := os.OpenFile(Conf.LogFilePath,os.O_APPEND|os.O_CREATE, 0644)
    if err != nil {
        fmt.Fprintf(os.Stderr,"Error %v\n", err)
        os.Exit(1)
    }

	BkStdOut := logging.NewLogBackend(os.Stderr, "", 0)
	BkLogFile := logging.NewLogBackend(file, "", 0)

	// the function.
	BkStdoutFormatter := logging.NewBackendFormatter(BkStdOut, format)

	// Only errors and more severe messages should be sent to backend1
	LogFileBkLevel := logging.AddModuleLevel(BkLogFile)
	LogFileBkLevel.SetLevel(logging.ERROR, "")

	// Set the backends to be used.
	logging.SetBackend(LogFileBkLevel, BkStdoutFormatter)
}

/**
 * @brief map domain, full path
 */
var FileMap map[string] string

/**
  * @brief Flag config
  */
var config_path = flag.String("c", "config.yaml", "Path to a config file")
var noPID = flag.Bool("n",false,"Not deploy PID file")

/**
 * @brief Configuration struct
 */
var Conf Config_t

/**
 * @brief Parse yaml config passed as flag parameter
 * @return False if found error
 */
func ParseConfig () {

    data, err := ioutil.ReadFile(*config_path)

    if err != nil {
        fmt.Fprintf(os.Stderr,"Impossible read file:%s Error%v", *config_path, err)
        os.Exit(1)
    }

    err = yaml.Unmarshal([]byte(data), &Conf)
    if err != nil {
        fmt.Fprintf(os.Stderr,"%v", err)
        os.Exit(1)
    }
}

/**
 * @brief Check if unix soket exist, and if file is Unix Soket
 * 
 */
func CheckUnixSoket(FullPath string) bool {
    FoundError := false

    // Check path exist
    _,err := os.Stat(FullPath)
    if err != nil {
        if os.IsNotExist(err) {
            /* Error is not fatal, soket could be removed, uWSGI restared, Then log it, and continue */
            log.Errorf("Could not open %s. This error is not critical will be SKIP", FullPath)
            FoundError = true
        } else {
            log.Fatalf("%v\r\n", err)
        }
    }
    // Let open File descriptor, check error
    if err != nil {
        FoundError = true
    }
    return FoundError
}

/**
 * @brief callback handler GET request
 */
func GET_Handling(w http.ResponseWriter, r *http.Request) {
    w.Write(ReadStatsSoket_uWSGI())
    w.Write([]byte(fmt.Sprintf("uwsgiexpoter_subroutine %d", runtime.NumGoroutine())))

}

/**
 * @brief Validate config file and fist open of FD
 */
func ValidateConfig () {
    FoundError := false
    FileMap = make(map[string] string)
    log.Info("Start check configuration file")

    _,err := ioutil.ReadDir(Conf.SoketDir)
    // Calculate full path
    // Fist validate soket dir path
    if err != nil {
        log.Fatalf("Error %v",err)
    }

    // Check path fist start polling
    for _, SoketPath := range(Conf.StatsSokets) {
        // Calculate full path
        FullPath := path.Join(Conf.SoketDir, SoketPath.Soket)

        if CheckUnixSoket(FullPath) {
            FoundError = true
        }
        FileMap[SoketPath.Domain] = FullPath
    }

    if !FoundError {
        log.Info("Configuration correct, no error detect")
    } else {
        log.Info("Error found check log")
    }
}

/**
 *
 * @brief Deploy pid file
 * Deploy pid file, Correct true, else false
 */
func DeployPID() bool {
    /**
     * TODO: Demonize
     * For do a good job this part must be demonizzed with double fork, write pid in /run/PIDNO
     * Find way to handle http with GIN or other lib to hangle reload, and restart signals
     */
    if *noPID {
        return true
    }

    // So ugly but it work
    PID := os.Getpid()

    pidFile,err := os.Open(Conf.PIDPath)
    if err != nil {
        log.Fatalf("Impossible open file, %s\r\n", err)
    }

    pidFile.WriteString(string(PID))
    pidFile.Sync()
    pidFile.Close()

    return true
}

func main() {
    // Setup env
	flag.Parse()
    ParseConfig()
    SetUpLogger()
    DeployPID()
    // End setup, from here all will be moved in second fork

    ValidateConfig()

    l, err := net.Listen("tcp", fmt.Sprintf(":%d", Conf.Port))
    if err != nil {
        log.Fatal(err)
    }

    log.Infof("Bin port:%d", Conf.Port)
    http.HandleFunc("/metrics", GET_Handling)

    http.Serve(l,nil)
}

