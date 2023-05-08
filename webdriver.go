package webdriver

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/tidwall/gjson"
)

//go:embed msedgedriver.exe
var msedgedriver []byte

const (
	// EdgeDriverPath is the path to the EdgeDriver executable.
	EdgeDriverPort = 30192
)

var userHomeDir, _ = os.UserHomeDir()

var DefaultEdgeDriverPath = userHomeDir + "/gowebdriver/msedgedriver.exe"
var SpareEdgeDriverPath = []string{
	"msedgedriver.exe",
	filepath.Join(currentPath(), "msedgedriver.exe"),
}

var edgeProcess *os.Process

func existFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func currentPath() string {
	file, err := exec.LookPath(os.Args[0])
	if err != nil {
		return ""
	}
	path, err := filepath.Abs(file)
	if err != nil {
		return ""
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		i = strings.LastIndex(path, "\\")
	}
	if i < 0 {
		return ""
	}
	return string(path[0 : i+1])
}

var DefaultResourcePort = 30193

func init() {
	if !existFile(DefaultEdgeDriverPath) {
		isInstall := false
		for i := range SpareEdgeDriverPath {
			if existFile(SpareEdgeDriverPath[i]) {
				DefaultEdgeDriverPath = SpareEdgeDriverPath[i]
				isInstall = true
				break
			}
		}

		if !isInstall {
			os.MkdirAll(userHomeDir+"/gowebdriver", 0777)
			os.WriteFile(DefaultEdgeDriverPath, msedgedriver, 0777)
		}
	}
	go func() {
		http.HandleFunc("/webdricer-static", func(w http.ResponseWriter, r *http.Request) {
			file := r.URL.Query().Get("f")
			if file == "" {
				w.WriteHeader(404)
				return
			}
			w.Write(filesResourceMap[file])
		})
		http.ListenAndServe("localhost:"+strconv.Itoa(DefaultResourcePort), nil)
	}()
}

type WebDriver struct {
	// contains filtered or unexported fields
	sessionId string
	debug     bool
}

func jsonEncode(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func (w *WebDriver) Println(v ...any) {
	if w.debug {
		log.Println(v...)
	}
}

func (w *WebDriver) SetDebug(debug bool) {
	w.debug = debug
}

// var logfile, _ = os.OpenFile("webdriver.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
// var logger = log.New(logfile, "webdriver: ", log.LstdFlags)

func (w *WebDriver) StartSession() error {

	go func() {
		cmd := exec.Command(DefaultEdgeDriverPath, "--port="+strconv.Itoa(EdgeDriverPort))
		edgeProcess = cmd.Process
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Run()
		// fmt.Println("stdout:", stdout.String())
	}()

	time.Sleep(500 * time.Millisecond)

	req := gorequest.New().Post("http://localhost:30192/session")

	args := []string{
		"--enable-automation",
		"--disable-gpu",
		"--disable-extensions",
		"--disable-infobars",
		"--disable-pop-up-blocking",
		"--disable-web-security",
		"--ignore-certificate-errors",
		"--disable-dev-shm-usage",
		"--disable-infobars",
		"--disable-extensions",
		"--disable-features=site-per-process",
		"--disable-hang-monitor",
		"--disable-ipc-flooding-protection",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-renderer-backgrounding",
		"--disable-sync",
		"--disable-translate",
		"--disable-windows10-custom-titlebar",
		"--metrics-recording-only",
		"--no-first-run",
		"--remote-allow-origins=*",
		"--no-default-browser-check",
		"--safebrowsing-disable-auto-update",
		"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36",
	}

	if !w.debug {
		args = append(args, "--headless")
	} else {
		args = append(args)
	}

	newSession := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": map[string]interface{}{
				"browserName":               "msedge",
				"strictFileInteractability": false,
				"acceptInsecureCerts":       false,
				"pageLoadStrategy":          "normal",
				"unhandledPromptBehavior":   "accept",
				"ms:edgeOptions": map[string]interface{}{
					"args": args,
				},
			},
		},
	}

	_, raw, errs := req.Send(jsonEncode(newSession)).End()
	if errs != nil {
		w.Println("StartSession", errs)
		return errs[0]
	}

	w.Println("StartSession:", raw)

	merr := gjson.Get(raw, "value.error").String()
	if merr != "" {
		return errors.New(fmt.Sprint(merr, "::", gjson.Get(raw, "value.message").String()))
	}

	w.sessionId = gjson.Get(raw, "value.sessionId").String()

	w.SetTimeout(300000)
	return nil
}

func (w *WebDriver) StopSession() {
	req := gorequest.New().Delete("http://localhost:30192/session/" + w.sessionId)
	_, raw, errs := req.End()
	if errs != nil {
		w.Println("StopSession:", errs)
		return
	}
	w.Println("StopSession:", raw)
	time.Sleep(500 * time.Millisecond)
}

func (w *WebDriver) Status() (string, error) {
	req := gorequest.New().Get("http://localhost:30192/status")
	_, raw, errs := req.End()
	if errs != nil {
		w.Println("Status:", errs)
		return "", errs[0]
	}

	w.Println("Status:", raw)
	return raw, nil
}

func (w *WebDriver) SetUrl(url string) (string, error) {
	req := gorequest.New().Post("http://localhost:30192/session/" + w.sessionId + "/url")
	_, raw, errs := req.Send(jsonEncode(map[string]string{"url": url})).End()
	if errs != nil {
		w.Println("Get:", errs)
		return "", errs[0]
	}
	w.Println("Get:", raw)
	return raw, nil
}

func (w *WebDriver) SetTimeout(timeout int) (string, error) {
	req := gorequest.New().Post("http://localhost:30192/session/" + w.sessionId + "/timeouts")
	_, raw, errs := req.Send(jsonEncode(map[string]int{
		"script":   timeout,
		"pageLoad": timeout,
		"implicit": timeout,
	})).End()
	if errs != nil {
		w.Println("SetTimeout:", errs)
		return "", errs[0]
	}
	w.Println("SetTimeout:", raw)
	return raw, nil
}

func (w *WebDriver) ExecuteScript(script string) (gjson.Result, error) {
	req := gorequest.New().Post("http://localhost:30192/session/" + w.sessionId + "/execute/sync")
	_, raw, errs := req.Send(jsonEncode(map[string]interface{}{
		"script": script,
		"args":   []interface{}{},
	})).End()
	if errs != nil {
		return gjson.Get(raw, "value"), errs[0]
	}
	return gjson.Get(raw, "value"), nil
}

var asyncTemplate = `
var callback = arguments[arguments.length - 1];
function sleep(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

async function asyncFunc() { 
	%s
}

var result = await asyncFunc()  
callback(result);
`

func (w *WebDriver) ExecuteAwaitScript(script string) (gjson.Result, error) {
	req := gorequest.New().Post("http://localhost:30192/session/" + w.sessionId + "/execute/async")
	_, raw, errs := req.Send(jsonEncode(map[string]interface{}{
		"script": fmt.Sprintf(asyncTemplate, script),
		"args":   []interface{}{},
	})).End()
	if errs != nil {
		return gjson.Get(raw, "value"), errs[0]
	}
	w.Println("ExecuteAwaitScript:", raw)
	return gjson.Get(raw, "value"), nil
}

var filesResourceMap = map[string][]byte{}
var filesResourceMapLock sync.Mutex

func (w *WebDriver) IncludeResourceFile(file string, isJs bool) {
	filesResourceMapLock.Lock()
	defer filesResourceMapLock.Unlock()
	fileRaw, _ := os.ReadFile(file)

	filesResourceMap[file] = fileRaw

	if isJs {
		w.ExecuteScript(`
		var script = document.createElement('script');
		script.src = 'http://localhost:` + strconv.Itoa(DefaultResourcePort) + `/webdricer-static?f=` + file + `';
		script.type = 'text/javascript';
		document.getElementsByTagName('head')[0].appendChild(script);
	`)
	}
}

func NewWebDriver() *WebDriver {

	return &WebDriver{}
}
