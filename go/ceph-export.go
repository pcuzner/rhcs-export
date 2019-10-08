package main

//
// export the ceph configuration
// code arbitrarily navigates through ceph -s json, instead of fully declaring
// a struct to define the whole of ceph -s output. This could be done later if
// desired
//

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v2"
)

// keyringFile is the filename pattern for a keyring
const keyringFile = "ceph.client.%s.keyring"

var defaults = map[string]string{
	"outFile":    "~/rhcs-export",
	"confDir":    "/etc/ceph",
	"fileFormat": "json",
	"userName":   "admin",
}

// Runtime settings
type runtimeSettings struct {
	outFile    string
	confDir    string
	fileFormat string
	userName   string
}

// exported ceph configuration metadata
type cephMetaData struct {
	DashboardURL  string   `json:"dashboard_url" yaml:"dashboard_url"`
	Fsid          string   `json:"fsid" yaml:"fsid"`
	Secret        string   `json:"secret" yaml:"secret"`
	Mgr           string   `json:"mgr" yaml:"mgr"`
	Mgrstandby    []string `json:"mgr_standby" yaml:"mgr_standby"`
	Mons          []string `json:"mons" yaml:"mons"`
	PrometheusURL string   `json:"prometheus_url" yaml:"prometheus_url"`
	Rgws          []string `json:"rgws" yaml:"rgws"`
	Version       string   `json:"version" yaml:"version"`
}

func isDir(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func isFile(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// look for string in a given slice
func hasString(item string, iterable []string) bool {
	for _, value := range iterable {
		if item == value {
			return true
		}
	}
	return false
}

// check if the environment is suitable for the export
func ready(settings *runtimeSettings) (bool, error) {

	keyring := fmt.Sprintf(keyringFile, settings.userName)
	keyringStore := settings.confDir + "/keyring-store/keyring"

	if !isDir(settings.confDir) {
		return false, errors.New("Directory '" + settings.confDir + "' not found")
	}

	if !isFile(settings.confDir + "/ceph.conf") {
		return false, errors.New("ceph configuration file missing from " + settings.confDir)
	}

	if !isFile(settings.confDir+"/"+keyring) && !isFile(keyringStore) {
		return false, errors.New("missing keyring/keyring store")
	}

	_, err := sendCommand("type ceph")
	if err != nil {
		return false, errors.New("ceph command is unavailable")
	}

	return true, nil
}

// exit the program with an error message
func abort(message string) {
	fmt.Printf("Unable to continue: %s\n", message)
	os.Exit(4)
}

// send a command to the OS, and return the response to the caller
func sendCommand(commandString string) (string, error) {
	args := strings.Split(commandString, " ")

	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("error running command")
	}
	return string(out), nil
}

// find the keyring for the given user and return its key
func fetchKeyring(userName string, confDir string) string {

	var keyFile string

	keyFilePath := confDir + "/" + fmt.Sprintf(keyringFile, userName)
	keyStore := confDir + "/keyring-store/keyring"
	if isFile(keyFilePath) {
		keyFile = keyFilePath
	} else if isFile(keyStore) {
		keyFile = keyStore
	}

	// what if keyFile is not set i.e. still empty?

	conf, err := getConfig(keyFile)
	if err != nil {
		return ""
	}
	keySection := conf.Section("client." + userName)
	key, err := keySection.GetKey("key")
	if err != nil {
		return ""
	}

	return key.String()
}

// simplistic hstname check -if ir starts with a number, it's an IP address!
func isIP(hostName string) bool {
	char1 := string(hostName[0])
	_, err := strconv.ParseInt(char1, 10, 8)
	if err != nil {
		return false
	}
	return true
}

// Read a ceph confg (ini) format
func getConfig(confFileName string) (*ini.File, error) {
	cfg, err := ini.Load(confFileName)
	if err != nil {
		return cfg, errors.New("Unable to load the config file")
	}
	return cfg, nil
}

// export to a file
func writeFile(output []byte, settings *runtimeSettings) {
	if strings.HasPrefix(settings.outFile, "~") {
		usr, _ := user.Current()
		settings.outFile = strings.Replace(settings.outFile, "~", usr.HomeDir, 1)
	}
	fileName := settings.outFile + "." + settings.fileFormat

	err := ioutil.WriteFile(fileName, output, 0644)
	if err != nil {
		fmt.Println(err)
		abort("Failed to write the file ")
	} else {
		fmt.Println("\nMetadata written to " + fileName)
	}
}

// dump to json
func toJSON(content *cephMetaData) []byte {

	out, err := json.MarshalIndent(content, "", "    ")
	if err != nil {
		abort("Export to json failed")
	}
	return out
}

// dump to yaml
func toYAML(content *cephMetaData) []byte {

	out := []byte("---\n")
	yaml, err := yaml.Marshal(content)
	if err != nil {
		abort("Export to yaml failed")
	}
	return append(out, yaml...)
}

// write ceph facts to a file
func exportMetadata(content *cephMetaData, settings *runtimeSettings) error {

	// fmt.Println(*content)
	// fmt.Println(*settings)
	switch settings.fileFormat {
	case "json":
		out := toJSON(content)
		writeFile(out, settings)
		// fmt.Println(string(out))
	case "yaml":
		out := toYAML(content)
		writeFile(out, settings)
	}

	return nil
}

func main() {

	var enabledModules []string
	var exportData cephMetaData

	// Defaults for the command line args
	outFile := flag.String("output", defaults["outFile"], "output file name")
	confDir := flag.String("confdir", defaults["confDir"], "Ceph configuration directory")
	fileFormat := flag.String("format", defaults["fileFormat"], "output file format")
	userName := flag.String("user", defaults["userName"], "user keyring")

	flag.Parse()
	settings := runtimeSettings{
		outFile:    *outFile,
		confDir:    *confDir,
		fileFormat: *fileFormat,
		userName:   *userName,
	}

	fmt.Print("\nChecking environment......")
	ok, err := ready(&settings)
	if !ok {
		fmt.Print("FAILED\n")
		abort(err.Error())
	} else {
		fmt.Print("PASSED\n")
	}

	key := fetchKeyring(settings.userName, settings.confDir)
	if key == "" {
		abort("Unable to load a key for the '" + *userName + "' user")
	}

	fmt.Print("Querying ceph state.......")
	cephStatusStr, err := sendCommand("ceph -s -f json")
	if err != nil {
		fmt.Print("FAILED")
		abort("Unable to gather status from ceph with 'ceph -s' command")
	} else {
		fmt.Print("OK\n")
	}

	bytes := []byte(cephStatusStr)
	var cephStatus map[string]interface{}
	err = json.Unmarshal(bytes, &cephStatus)
	if err != nil {
		abort("Unable to parse the json output from Ceph!")
	}

	fmt.Print("Checking ceph version.....")
	cephVersOutput, err := sendCommand("ceph --version")
	if err != nil {
		fmt.Print("FAILED\n")
		abort("failed trying to extract ceph version from the system")
	}

	// the first 13 chars are 'ceph version ', so let's skip those!
	exportData.Version = strings.Split(cephVersOutput[13:], "-")[0]
	if !strings.HasPrefix(exportData.Version, "14") {
		abort("Export utility only supported on Nautilus clusters")
	} else {
		fmt.Print("PASSED\n")
	}

	for idx, k := range cephStatus {

		switch idx {
		case "monmap":
			// fmt.Println("Processing monmap")
			for midx, mval := range k.(map[string]interface{}) {
				switch midx {
				case "mons":
					// fmt.Println("processing mons")
					for _, monData := range mval.([]interface{}) {
						monIP := monData.(map[string]interface{})["addr"]
						exportData.Mons = append(exportData.Mons, monIP.(string))
					}
				}
			}
		case "mgrmap":
			// fmt.Println("Processing mgrmap")
			for mgrKey, mgrVal := range k.(map[string]interface{}) {

				switch mgrKey {
				case "active_addr":
					exportData.Mgr = strings.Split(mgrVal.(string), ":")[0]
				case "standbys":
					for _, stdbyData := range mgrVal.([]interface{}) {
						s := stdbyData.(map[string]interface{})
						mgrName := s["name"].(string)
						if !isIP(mgrName) {
							ip, err := net.LookupHost(mgrName)
							if err == nil {
								mgrName = ip[0]
							}
						}
						exportData.Mgrstandby = append(exportData.Mgrstandby, mgrName)
					}
				case "modules":
					for _, mod := range mgrVal.([]interface{}) {
						enabledModules = append(enabledModules, mod.(string))
					}
				case "services":
					for svcName, svcURL := range mgrVal.(map[string]interface{}) {
						switch svcName {
						case "dashboard":
							exportData.DashboardURL = svcURL.(string)
						case "prometheus":
							exportData.PrometheusURL = svcURL.(string)
						}
					}
				}
			}
		case "servicemap":
			svcMap := k.(map[string]interface{})
			svcs := svcMap["services"].(map[string]interface{})

			if rgw, ok := svcs["rgw"]; ok {
				rgwDaemons := rgw.(map[string]interface{})["daemons"]
				for rgwKey, rgwData := range rgwDaemons.(map[string]interface{}) {
					if rgwKey == "summary" {
						continue
					}
					rgwMeta := rgwData.(map[string]interface{})["metadata"]
					frontEnd := rgwMeta.(map[string]interface{})["frontend_config#0"].(string)
					fendSettings := strings.Split(frontEnd, " ")
					for _, item := range fendSettings {
						parms := strings.Split(item, "=")
						if strings.HasSuffix(parms[0], "port") {
							exportData.Rgws = append(exportData.Rgws, parms[1])
						}
					}
				}
			}

		}

	}

	if !hasString("prometheus", enabledModules) {
		abort("Prometheus module must be enabled, prior to configuration export")
	}
	fmt.Println("Active mgr module check...PASSED")

	exportData.Secret = key
	exportData.Fsid = cephStatus["fsid"].(string)

	exportMetadata(&exportData, &settings)
}
