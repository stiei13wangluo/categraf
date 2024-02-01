package netdev_snmp

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flashcat.cloud/categraf/config"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func getOidIndex(oid string) string {
	oidSlice := strings.Split(oid, ".")
	oidLen := len(oidSlice)
	return oidSlice[oidLen-1]
}

func splitString(s string, sep string) []string {
	var res []string
	start := 0
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], sep) {
			res = append(res, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	res = append(res, s[start:])
	return res
}

func findStrMin(arr []string) int {

	min, err := strconv.Atoi(strings.Trim(arr[0], "\""))

	if err != nil {
		panic(err)
	}

	for _, v := range arr {
		i, err := strconv.Atoi(strings.Trim(v, "\""))
		if err != nil {
			panic(err)
		}

		if i < min {
			min = i
		}

	}
	return min
}

const collinterval = 3

func Work(hostname string) {

	fmt.Printf(hostname)
	conf := config.Config.Heartbeat

	if conf == nil || !conf.Enable {
		return
	}

	version := config.Version
	versions := strings.Split(version, "-")
	if len(versions) > 1 {
		version = versions[0]
	}

	//ps := system.NewSystemPS()

	interval := conf.Interval
	if interval <= 4 {
		interval = 4
	}

	client, err := newHTTPClient()
	if err != nil {
		log.Println("E! failed to create heartbeat client:", err)
		return
	}

	work(version, hostname, client)

	//duration := time.Second * time.Duration(interval-collinterval)
	//
	//for {
	//	work(version, hostname, client)
	//	time.Sleep(duration)
	//}
}

func newHTTPClient() (*http.Client, error) {
	proxy, err := config.Config.Heartbeat.Proxy()
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(config.Config.Heartbeat.Timeout) * time.Millisecond

	trans := &http.Transport{
		Proxy: proxy,
		DialContext: (&net.Dialer{
			Timeout: time.Duration(config.Config.Heartbeat.DialTimeout) * time.Millisecond,
		}).DialContext,
		ResponseHeaderTimeout: timeout,
		MaxIdleConnsPerHost:   config.Config.Heartbeat.MaxIdleConnsPerHost,
	}

	if strings.HasPrefix(config.Config.Heartbeat.Url, "https:") {
		tlsCfg, err := config.Config.Heartbeat.TLSConfig()
		if err != nil {
			log.Println("E! failed to init tls:", err)
			return nil, err
		}

		trans.TLSClientConfig = tlsCfg
	}

	client := &http.Client{
		Transport: trans,
		Timeout:   timeout,
	}

	return client, nil
}

func work(version string, hostname string, client *http.Client) {
	//cpuUsagePercent := cpuUsage(ps)
	//hostname := config.Config.GetHostname()
	//fmt.Printf("%v\n", hostname)
	//memUsagePercent := memUsage(ps)

	//cpuUsagePercent := 60
	//memUsagePercent := 60

	data := map[string]interface{}{
		//"agent_version": version,
		//"os":            "",
		//"arch":          "",
		"hostname": hostname,
		//"cpu_num":       "",
		//"cpu_util": cpuUsagePercent,
		//"mem_util": memUsagePercent,
		"unixtime": time.Now().UnixMilli(),
	}

	fmt.Printf("%v\n", time.Now().UnixMilli())

	bs, err := json.Marshal(data)
	if err != nil {
		log.Println("E! failed to marshal heartbeat request:", err)
		return
	}

	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	if _, err = g.Write(bs); err != nil {
		log.Println("E! failed to write gzip buffer:", err)
		return
	}

	if err = g.Close(); err != nil {
		log.Println("E! failed to close gzip buffer:", err)
		return
	}

	req, err := http.NewRequest("POST", config.Config.Heartbeat.Url, &buf)
	if err != nil {
		log.Println("E! failed to new heartbeat request:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("User-Agent", "categraf/"+version)

	for i := 0; i < len(config.Config.Heartbeat.Headers); i += 2 {
		req.Header.Add(config.Config.Heartbeat.Headers[i], config.Config.Heartbeat.Headers[i+1])
		if config.Config.Heartbeat.Headers[i] == "Host" {
			req.Host = config.Config.Heartbeat.Headers[i+1]
		}
	}

	if config.Config.Heartbeat.BasicAuthPass != "" {
		req.SetBasicAuth(config.Config.Heartbeat.BasicAuthUser, config.Config.Heartbeat.BasicAuthPass)
	}

	//fmt.Printf("%v\n", req.Body))

	res, err := client.Do(req)
	if err != nil {
		log.Println("E! failed to do heartbeat:", err)
		return
	}

	if res.StatusCode/100 != 2 {
		log.Println("E! heartbeat status code:", res.StatusCode)
		return
	}

	defer res.Body.Close()
	bs, err = ioutil.ReadAll(res.Body)
	fmt.Printf("%v\n", string(bs))

	if err != nil {
		log.Println("E! failed to read heartbeat response body:", err)
		return
	}

	if config.Config.DebugMode {
		log.Println("D! heartbeat response:", string(bs), "status code:", res.StatusCode)
	}
}
