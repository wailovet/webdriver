package webdriver

import (
	"log"
	"testing"
	"time"
)

func TestNewWebDriver(t *testing.T) {
	wd := NewWebDriver()
	wd.SetDebug(false)

	wd.StartSession()
	defer wd.StopSession()

	wd.SetUrl("https://www.baidu.com")

	title, err := wd.ExecuteAwaitScript(`
		await sleep(1000);
		console.log('title:', document.title);
		return document.title;
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Println("title:", title)
	time.Sleep(100 * time.Second)

}
