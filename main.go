package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"bufio"


	pd "github.com/shenyi-tw/golib/dao/proxy"
	logger "github.com/shenyi-tw/golib/log"
	"golang.org/x/net/proxy"
)

var (
	DatabaseHost     string
	DatabasePort     string
	DatabaseName     string
	DatabaseUser     string
	DatabasePassword string
	MaxDatabaseRetry string
	ThreadNum        string
)

func init() {
	DatabaseHost = os.Getenv("DatabaseHost")
	DatabasePort = os.Getenv("DatabasePort")
	DatabaseName = os.Getenv("DatabaseName")
	DatabaseUser = os.Getenv("DatabaseUser")
	DatabasePassword = os.Getenv("DatabasePassword")
	MaxDatabaseRetry = os.Getenv("MaxDatabaseRetry")
	ThreadNum = os.Getenv("ThreadNum")
}

const TestUrl = "https://www.google.com/"

func CheckProxy(client *http.Client) (isProxy bool, err error) {
	res, err := client.Get(TestUrl)
	if err != nil {
		return false, err
	} else {
		defer res.Body.Close()
		if res.StatusCode == 200 {
			body, err := ioutil.ReadAll(res.Body)
			if err == nil && strings.Contains(string(body), "google") {
				return true, nil
			} else {
				return false, err
			}
		} else {
			return false, nil
		}
	}
}

func IsProxy(proxyAddr string) (isProxy bool, err error) {
	dialSocksProxy, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return false, nil
	}
	netTransport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
		c, e := dialSocksProxy.Dial(network, addr)
		return c, e
	}}
	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}
	return CheckProxy(client)
}

func main() {
	createProxies()
	checkProxy()
}


// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}


func createProxies() {
	lines, err := readLines("./proxy.txt")
	fmt.Println("len(lines)", len(lines))
	if err != nil {
		return
	}
	conn := pd.CreateConn(DatabaseUser, DatabasePassword, DatabaseHost, DatabasePort, DatabaseName)

	conn.CreateProxies(lines)
	conn.Close()
}

func checkProxy() {

	threadNum, err := strconv.ParseInt(ThreadNum, 10, 64)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("%s", err))
		return
	}
	conn := pd.CreateConn(DatabaseUser, DatabasePassword, DatabaseHost, DatabasePort, DatabaseName)

	prox, _ := conn.GetProxyAll()

	wg := sync.WaitGroup{}
	ch := make(chan int, threadNum)
	for i := 0; i < int(threadNum); i++ {
		go func(i int) {
			for idx := range ch {
				if i == 0 {
					logger.Log("DEBUG", fmt.Sprintf("%s", prox[idx].Addr))
				}
				flag, _ := IsProxy(prox[idx].Addr)
				if flag {
					prox[idx].Success += 1
					for i := 0; i < 5; i++ {
						_, err := pd.HealthP(prox[idx].Addr)
						if err == nil {
							prox[idx].SuccessPtt += 1
							prox[idx].LastSuc = time.Now()
							logger.Log("INFO", fmt.Sprintf("success %s", prox[idx].Addr))
							break
						}
					}
				} else {
					prox[idx].Fail += 1
					if prox[idx].Fail > 48 {
						if prox[idx].SuccessPtt*3 < prox[idx].Fail {
							prox[idx].Active = false
						}
					}
				}
				wg.Done()
			}
		}(i)
	}
	for idx := range prox {
		wg.Add(1)
		ch <- idx
	}
	wg.Wait()
	// fmt.Println(prox)
	conn.SaveProxy(prox)
	conn.Close()
}
