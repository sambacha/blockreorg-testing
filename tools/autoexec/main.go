package main

import (
	_ "bufio"
	"encoding/json"
	"fmt"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	_ "net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type stats struct {
	Max               float64 `json:"max"`
	StandardDeviation float64 `json:"standardDeviation"`
	Mean              float64 `json:"mean"`
}

type jsonStruct struct {
	Difficulty          stats   `json:"difficulty"`
	TotalDifficulty     stats   `json:"totalDifficulty"`
	GasLimit            stats   `json:"gasLimit"`
	GasUsed             stats   `json:"gasUsed"`
	BlockTime           stats   `json:"blockTime"`
	BlockSize           stats   `json:"blockSize"`
	TransactionPerBlock stats   `json:"transactionPerBlock"`
	UncleCount          stats   `json:"uncleCount"`
	Tps                 stats   `json:"tps"`
	Blocks              float64 `json:"blocks"`
}

type TestEnv struct {
	HostName  string     `json:"hostName"`
	TestName  string     `json:"testName"`
	TimeBegin int64      `json:"timeBegin"`
	TimeEnd   int64      `json:"timeEnd"`
	TestID    string     `json:"testID"`
	WebStats  jsonStruct `json:"webStats"`
}

type SysEnv struct {
	timeBegin    int64
	timeEnd      int64
	hostName     string
	fileName     string
	testName     string
	testID       string
	UserName     string
	webDataURL   string
	pathSyslogNG string
	efLog        string
	pathLog      string
	autoExecLog  string
	pathYaml     string
	externalIP   string
	webStats     jsonStruct
	rstatsPID    int
}

func (test *SysEnv) setDefaults() error {
	test.hostName, _ = os.Hostname()
	test.fileName = ""
	test.testID = ""
	test.webDataURL = ""
	test.pathSyslogNG = "/var/log/syslog-ng/"
	test.efLog = "ef-test.log"
	test.pathLog = "autoexec-log/"
	test.autoExecLog = "autoexec.log"
	test.pathYaml = "/var/log/syslog-ng/ef-testing/autoexec-yaml"
	test.externalIP = ""
	/*	test.webStats	  = `{"difficulty":{"max":55000000,"standardDeviation":427516.00232242176,"mean":51202982.53106213},"totalDifficulty":{"max":25550288283,"standardDeviation":7391818514.634528,"mean":12768191452.43888},"gasLimit":{"max":11850000,"standardDeviation":595.7486135081332,"mean":11849961.018036073},"gasUsed":{"max":11371336,"standardDeviation":508257.5740591193,"mean":11342176.352705412},"blockTime":{"max":631,"standardDeviation":30.57527317048098,"mean":13.160965794768613},"blockSize":{"max":163758,"standardDeviation":7276.488035583049,"mean":162884.8316633266},"transactionPerBlock":{"max":25,"standardDeviation":1.1180317437084863,"mean":24.949899799599194},"uncleCount":{"max":1,"standardDeviation":0.09959738388608554,"mean":0.010020040080160317},"tps":{"max":25,"standardDeviation":7.716263384080033,"mean":6.520443852228358},"blocks":499}`*/
	/*	test.webStats	  = ""*/

	/*
	   	nglog_file := test.pathSyslogNG+test.efLog
	   	if _, err := os.Stat(nglog_file); os.IsNotExist(err) {
	       	f, _ := os.Create(nglog_file)
	       	f.Close()
	   	}
	*/
	autoexeclog_file := test.pathSyslogNG + test.pathLog + test.autoExecLog
	if _, err := os.Stat(autoexeclog_file); os.IsNotExist(err) {
		err := os.Mkdir(test.pathSyslogNG+test.pathLog, 0755)
		err = ioutil.WriteFile(autoexeclog_file, []byte(""), 0644)
		if err != nil {
			log.WithFields(log.Fields{"file": autoexeclog_file, "error": err}).Error("Unable to create autoexeclog_file file .")
			return fmt.Errorf("Unable to create autoexeclog_file file.")
		}
	}

	return nil
}

func (test *SysEnv) getGenesisUserName() error {
	cmd := exec.Command("genesis", "whoami", "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": err}).Error("Unable to call genesis whoami.")
		return fmt.Errorf("Unable to call genesis whoami.")
	}
	json.Unmarshal([]byte(out), &test)

	cmd = exec.Command("genesis", "org")
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": err}).Error("Unable to call genesis org.")
		return fmt.Errorf("Unable to call genesis org.")
	}
	test.UserName = strings.TrimSpace(string(out))

	return nil
}

func getExternalIP() (string, error) {
	cmd := exec.Command("curl", "-s", "ifconfig.so")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": err}).Error("Unable to set external IP.")
		return "", fmt.Errorf("Unable to set genesis external IP.")

	} else {
		return string(out), nil
	}
}

func setSyslogng(ip string) error {

	cmd := exec.Command("genesis", "settings", "set", "syslogng-host", ip)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"ip_address": ip, "out": string(out), "cmd": cmd, "error": err}).Error("Unable to set genesis syslogng host.")
		return fmt.Errorf("Unable to set genesis syslogng host.", ip)

	} else {
		return nil
	}
}

func getYamlFiles(file_path string) []string {

	var files []string

	err := filepath.Walk(file_path, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}

func (test *SysEnv) getTestDNS() error {
	// wait 8 mins for DNS to become available
	time.Sleep(600 * time.Second)
	cmd := exec.Command("genesis", "info", test.testID, "--json")
	fmt.Println(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"test_ID": test.testID, "out": string(out), "cmd": cmd, "error": err}).Error("Unable to get test DNS.")
		return fmt.Errorf("Unable to get test DNS.")

	}

	var result map[string]interface{}
	json.Unmarshal([]byte(out), &result)
	instance := result["instances"].([]interface{})
	domain := instance[0].(map[string]interface{})
	test.webDataURL = domain["domain"].(string) + ".biomes.whiteblock.io:8080/stats/all"
	return nil
}

func (test *SysEnv) getTestId() error {
	cmd := exec.Command("genesis", "tests", "-l", "-1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"ip_address": test.externalIP, "out": string(out), "cmd": cmd, "error": err}).Error("Unable to set genesis syslogng host.")
		return fmt.Errorf("Unable to set genesis syslogng host.")

	}
	// strip the new line character from the test id
	test.testID = strings.TrimSuffix(string(out), "\n")
	return nil
}

func (test *SysEnv) monitorWebData() error {
	statsURL := "http://" + test.webDataURL
	startRstats := 0
	for {
		resp, err := http.Get(statsURL)
		if resp == nil {
			log.WithFields(log.Fields{"endpoint": statsURL, "resp": resp, "error": err}).Info("Unable to make http request, trying again.")
			time.Sleep(15 * time.Second)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if string(body) != "" {
			if startRstats == 0 {
				// Start RSTATS data collection
				ok := test.startRstats()
				if ok != nil {
					log.WithFields(log.Fields{"error": ok}).Error("Unable to start RSTATS collection.")
					return fmt.Errorf("Unable to start RSTATS process.")
				}

				startRstats = 1
			}

			var result = jsonStruct{}
			json.Unmarshal([]byte(body), &result)
			test.webStats = result

			split := strings.Split(test.fileName, "/")
			slice_file := strings.Split(split[len(split)-1], ".")

			time_now := time.Now()
			logStats := TestEnv{}
			logStats.HostName = test.hostName
			logStats.TestName = slice_file[0]
			logStats.TimeBegin = test.timeBegin
			logStats.TimeEnd = time_now.Unix()
			logStats.TestID = test.testID
			logStats.WebStats = test.webStats

			jsonTest, _ := json.Marshal(logStats)
			fmt.Println(string(jsonTest))

			if result.Blocks > 420 {
				// set test.webStats to write to file when test is done
				resp.Body.Close()
				break
			}

			resp.Body.Close()
		}
		time.Sleep(15 * time.Second)
	}
	return nil
}

func (test *SysEnv) startRstats() error {
	// genesis stats cad --json -t 670e1118-5620-4c91-8560-eafd14c73048  >> /var/log/syslog-ng/autoexec-log/670e1118-5620-4c91-8560-eafd14c73048.stats
	file_name := test.pathSyslogNG + test.pathLog + test.testName + "_" + test.testID + ".stats"
	file, err := os.OpenFile(file_name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.WithFields(log.Fields{"file_name": file_name, "error": err}).Error("Unable to open RSTATS file.")
		return fmt.Errorf("Unable to start RSTATS process.")
	}
	cmd := exec.Command("genesis", "stats", "cad", "--json", "-t", test.testID)
	cmd.Stdout = file
	cmd.Stderr = file
	err = cmd.Start()
	if err != nil {
		log.WithFields(log.Fields{"cmd": cmd, "file_name": file_name, "error": err}).Error("Unable to start RSTATS function.")
		return fmt.Errorf("Unable to start RSTATS function.")
	}
	test.rstatsPID = cmd.Process.Pid

	return nil
}

func (test *SysEnv) beginTest() error {
	/*	cmd := exec.Command("genesis", "run", test.fileName, "paccode", "--no-await", "--json")*/
	cmd := exec.Command("genesis", "run", test.fileName, test.UserName, "--no-await", "--json")
	fmt.Println(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{"file": test.fileName, "out": string(out), "cmd": cmd, "error": err}).Error("Unable to start genesis run host.")
		return fmt.Errorf("Unable to start genesis run host.")
	}
	fmt.Println(string(out))
	return nil
}

func (test *SysEnv) cleanUp(test_err int) error {
	// stop current genesis test
	cmd := exec.Command("genesis", "stop", test.testID)
	fmt.Println(cmd)
	out, ok := cmd.CombinedOutput()
	if ok != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to stop current genesis test.")
		return fmt.Errorf("Unable to stop current genesis test: " + test.testID)

	}

	// give the NG && RSTATS logs 30 seconds to catch up
	time.Sleep(30 * time.Second)

	// kill RSTATS collection
	// ***********************
	ok = syscall.Kill(test.rstatsPID, syscall.SIGKILL)
	if ok != nil {
		log.WithFields(log.Fields{"error": ok}).Error("Unable to kill RSTATS process.")
		return fmt.Errorf("Unable to kill RSTATS process TESTID: " + test.testID)

	}
	// if err == 0 copy syslogng-logs to test-ef/ directory
	// cp ef-test.log test-ef/ef-test-670e1118.log
	if test_err == 0 {
		splitID := strings.Split(test.testID, "-")
		logFile := test.pathSyslogNG + test.efLog
		statsFile := test.pathSyslogNG + test.pathLog + test.testName + "_ef-test-" + splitID[0] + ".log"
		ok := os.Rename(logFile, statsFile)
		if ok != nil {
			log.WithFields(log.Fields{"logFile": logFile, "statsFile": statsFile, "error": ok}).Error("Unable to copy NGlog data to stats directory.")
			return fmt.Errorf("Unable to copy NGlog data to stats directory: " + test.testID)

		}
		// restart syslog-ng
		// systemctl restart syslog-ng
		cmd = exec.Command("systemctl", "restart", "syslog-ng")
		fmt.Println(cmd)
		out, ok = cmd.CombinedOutput()
		if ok != nil {
			log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to restart syslog-ng for current genesis test: " + test.testID)
			return fmt.Errorf("Unable to restart syslog-ng for current genesis test: " + test.testID)

		}
		/*
			cmd = exec.Command("touch", test.pathSyslogNG+test.efLog)
			fmt.Println(cmd)
			out, ok = cmd.CombinedOutput()
			if ok != nil {
				log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to create new NG log file for current genesis test: "+test.testID)
				return fmt.Errorf("Unable to create new NG log file for current genesis test: "+test.testID)

			}
			cmd = exec.Command("chmod", "777", test.pathSyslogNG+test.efLog)
			fmt.Println(cmd)
			out, ok = cmd.CombinedOutput()
			if ok != nil {
				log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to create new NG log file for current genesis test: "+test.testID)
				return fmt.Errorf("Unable to create new NG log file for current genesis test: "+test.testID)

			}
		*/
	} else { // there must have been an error so clear the NG log data

		// clear data from current syslog-ng/ef-test.log file
		cmd = exec.Command(">", test.pathSyslogNG+test.efLog)
		fmt.Println(cmd)
		out, ok = cmd.CombinedOutput()
		if ok != nil {
			log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to clear NG logs for current genesis test: " + test.testID)
			return fmt.Errorf("Unable to clear NG logs for current genesis test: " + test.testID)

		}
	}

	// Write test.webStats data to file
	file, ok := os.OpenFile(test.pathSyslogNG+test.pathLog+test.autoExecLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ok != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to open autoexec log for current genesis test: " + test.testID)
		return fmt.Errorf("Unable to open autoexec log for current genesis test: " + test.testID)
	}
	/*
		split := strings.Split(test.fileName, "/")
		slice_file := strings.Split(split[len(split)-1], ".")
	*/

	time_now := time.Now()
	logStats := TestEnv{}
	logStats.HostName = test.hostName
	logStats.TestName = test.testName
	logStats.TimeBegin = test.timeBegin
	logStats.TimeEnd = time_now.Unix()
	logStats.TestID = test.testID
	logStats.WebStats = test.webStats

	jsonTest, _ := json.Marshal(logStats)
	fmt.Println(string(jsonTest))
	line := string(jsonTest) + "\n"
	if _, err := file.WriteString(line); err != nil {
		log.WithFields(log.Fields{"out": string(out), "cmd": cmd, "error": ok}).Error("Unable to write to autoexec log for current genesis test: " + test.testID)
		return fmt.Errorf("Unable to write to autoexec log for current genesis test: " + test.testID)
	}

	file.Close()
	return nil
}

func main() {
	mail_subject := ""
	err := error(nil)
	var test = SysEnv{}
	err = test.setDefaults()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Unable to set/get default values. exiting now.")
		return
	}
	test.externalIP, err = getExternalIP()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Unable to get external IP exiting now.")
		return
	}
	// set genesis username for this server
	test.getGenesisUserName()

	file := getYamlFiles(test.pathYaml)
	for i := 1; i < len(file); i++ {
		// Set genesis settings set syslogng-host
		err = setSyslogng(test.externalIP)
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "External IP": test.externalIP, "error": err}).Error("Unable to set syslogng-host genesis paramater.")
			return
		}
		time_now := time.Now()
		test.timeBegin = time_now.Unix()
		test.fileName = file[i]
		split := strings.Split(test.fileName, "/")
		slice_file := strings.Split(split[len(split)-1], ".")
		test.testName = slice_file[0]
		fmt.Println("test ", i, "Begin --- ", test)

		// Error subject line for email sent in case of error for any of the calls below
		mail_subject = "Error running test: " + test.testName + " from " + test.hostName + " server"
		// Begin genesis run yaml_file test
		err = test.beginTest()
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "error": err}).Error("Unable to begin test.")
			test.sendEmail(mail_subject, err.Error())
			return
		}
		// Get test ID
		err = test.getTestId()
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "error": err}).Error("Unable to get test id.")
			test.sendEmail(mail_subject, err.Error())
			return
		}
		// Get test DNS so we can monitor web stats
		err = test.getTestDNS()
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "error": err}).Error("Unable to get test web data URL.")
			test.sendEmail(mail_subject, err.Error())
			return
		}
		// Start Webdata collection
		// this function start RSTATS collection as soon as webstats start to come in
		err = test.monitorWebData()
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "error": err}).Error("Unable to monitor web stats.")
			test.sendEmail(mail_subject, err.Error())
			return
		}
		fmt.Println(test)
		// Test has finished cleanup and get ready for the next test
		err = test.cleanUp(0)
		if err != nil {
			log.WithFields(log.Fields{"yaml_file": file, "error": err}).Error("Unable to clean up after test.")
			test.sendEmail(mail_subject, err.Error())
			return
		}
		mail_subject = "Successfully completed test: " + test.testName + " from " + test.hostName + " server"
		test.sendEmail(mail_subject, "")
		fmt.Println(file[i])
	}
}

func (test *SysEnv) sendEmail(subject string, message string) {

	type outStruct struct {
		HostName  string     `json:"hostName"`
		TestName  string     `json:"testName"`
		TimeBegin int64      `json:"timeBegin"`
		TimeEnd   int64      `json:"timeEnd"`
		TestID    string     `json:"testID"`
		WebStats  jsonStruct `json:"webStats"`
		ErrorData string     `json:"errorData"`
	}

	outData := outStruct{}
	outData.HostName = test.hostName
	outData.TestName = test.testName
	outData.TimeBegin = test.timeBegin
	outData.TimeEnd = test.timeEnd
	outData.TestID = test.testID
	outData.WebStats = test.webStats
	outData.ErrorData = message
	// create new *SGMailV3
	m := mail.NewV3Mail()

	jsonData, _ := json.Marshal(outData)

	from := mail.NewEmail(test.hostName, "bill@whiteblock.io")
	content := mail.NewContent("text/html", "<p> "+string(jsonData)+"</p>")

	m.SetFrom(from)
	m.AddContent(content)

	// create new *Personalization
	personalization := mail.NewPersonalization()

	// populate `personalization` with data
	to1 := mail.NewEmail("Bill Hamilton", "bill@whiteblock.io")

	to2 := mail.NewEmail("Jason Tran", "jason@whiteblock.io")
	to3 := mail.NewEmail("Nate Blakely", "nate@whiteblock.io")

	personalization.AddTos(to1, to2, to3)
	personalization.Subject = subject

	// add `personalization` to `m`
	m.AddPersonalizations(personalization)

	request := sendgrid.GetRequest("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", "/v3/mail/send", "https://api.sendgrid.com")
	request.Method = "POST"
	request.Body = mail.GetRequestBody(m)
	response, err := sendgrid.API(request)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Println(response.StatusCode)
		fmt.Println(response.Body)
		fmt.Println(response.Headers)
	}
}
func init() {
	// Log as JSON instead of the default ASCII formatter.
	/*	log.SetFormatter(&log.JSONFormatter{})*/

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	/*	log.SetOutput(os.Stdout)*/

	log.SetReportCaller(true)
	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
}
