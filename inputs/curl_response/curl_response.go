package curl_response

import "C"
import (
	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/types"
	"github.com/sashayakovtseva/go-curl"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	inputName = "curl_response"
)

type Instance struct {
	config.InstanceConfig

	Urls []string `toml:"urls"`

	// Dns query timeout in seconds. 0 means no timeout
	Timeout int `toml:"timeout"`
}

type CurlResponse struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

type ResultResponse struct {
	InfoResult            int
	SSLResult             int
	SSLExpire             int
	InfoResponseCode      int
	InfoTotalTime         float64
	InfoNamelookupTime    float64
	InfoConnectTime       float64
	InfoPretransferTime   float64
	InfoStarttransferTime float64
}

func init() {
	inputs.Add("curl_response", func() inputs.Input {
		return &CurlResponse{}
	})
}

func (cr *CurlResponse) Clone() inputs.Input {
	return &CurlResponse{}
}

func (cr *CurlResponse) Name() string {
	return inputName
}

func (cr *CurlResponse) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(cr.Instances))
	for i := 0; i < len(cr.Instances); i++ {
		ret[i] = cr.Instances[i]
	}
	return ret
}

func (ins *Instance) DoCurl(url string) ResultResponse {
	curl.GlobalInit(curl.GLOBAL_DEFAULT)
	defer curl.GlobalCleanup()
	easy := curl.EasyInit()
	defer easy.Cleanup()

	easy.Setopt(curl.OPT_URL, url)
	easy.Setopt(curl.OPT_SSL_VERIFYPEER, false)
	easy.Setopt(curl.OPT_SSL_VERIFYHOST, false)
	easy.Setopt(curl.OPT_TIMEOUT, ins.Timeout)
	easy.Setopt(curl.OPT_CERTINFO, true)
	easy.Setopt(curl.OPT_NOPROGRESS, true)
	easy.Setopt(curl.OPT_NOBODY, true)
	easy.Setopt(curl.OPT_FORBID_REUSE, true)
	easy.Setopt(curl.OPT_FRESH_CONNECT, true)
	easy.Setopt(curl.OPT_USERAGENT, "Mozilla/5.2 (compatible; MSIE 6.0; Windows NT 5.1; SV1; .NET CLR 1.1.4322; .NET CLR 2.0.50324)")

	// make a callback function
	curlReq := func(buf []byte, userdata interface{}) bool {
		//println("DEBUG: size=>", len(buf))
		//println("DEBUG: content=>", string(buf))
		return true
	}

	easy.Setopt(curl.OPT_WRITEFUNCTION, curlReq)

	result := ResultResponse{}

	if err := easy.Perform(); err != nil {

		// result 0: 连接失败 1：正常 2: 连接超时 3：DNS解析失败 4: 接收异常 -1：未知异常
		if strings.Contains(err.Error(), "connect to server") {
			result.InfoResult = 0
		} else if strings.Contains(err.Error(), "Timeout was reached") {
			result.InfoResult = 2
		} else if strings.Contains(err.Error(), "resolve host name") {
			result.InfoResult = 3
		} else if strings.Contains(err.Error(), "Failure when receiving data from the peer") {
			result.InfoResult = 4
		} else {
			result.InfoResult = -1
			log.Println("E! failed to preform request: ", err)
		}
		return result
	}

	result.InfoResult = 1

	//RESPONSE_CODE
	info, err := easy.Getinfo(curl.INFO_RESPONSE_CODE)
	if err == nil {
		result.InfoResponseCode = info.(int)
	}

	//TOTAL_TIME
	info, err = easy.Getinfo(curl.INFO_TOTAL_TIME)
	if err == nil {
		result.InfoTotalTime = Decimal(info.(float64))
	}

	//NAMELOOKUP_TIME
	info, err = easy.Getinfo(curl.INFO_NAMELOOKUP_TIME)
	if err == nil {
		result.InfoNamelookupTime = Decimal(info.(float64))
	}

	//CONNECT_TIME
	info, err = easy.Getinfo(curl.INFO_CONNECT_TIME)
	if err == nil {
		result.InfoConnectTime = Decimal(info.(float64))
	}

	// PRETRANSFER_TIME
	info, err = easy.Getinfo(curl.INFO_PRETRANSFER_TIME)
	if err == nil {
		result.InfoPretransferTime = Decimal(info.(float64))
	}

	// STARTTRANSFER_TIME
	info, err = easy.Getinfo(curl.INFO_STARTTRANSFER_TIME)
	if err == nil {
		result.InfoStarttransferTime = Decimal(info.(float64))
	}

	if strings.Contains(url, "https://") {

		//SSLResult -1 无数据 0 异常 1 正常

		// 验证是否错误
		info, err = easy.Getinfo(curl.INFO_SSL_VERIFYRESULT)
		sslVerify := info
		if sslVerify != 0 {
			result.SSLResult = 0
			//return result
		}

		info, err = easy.Getinfo(curl.INFO_CERTINFO)

		if err != nil {
			result.SSLResult = 0
			return result
		}

		certInfo := info.([]string)[0]
		//Linux: m := regexp.MustCompile(`\nExpire Date:(?P<expire_date>.*)\n`)
		m := regexp.MustCompile(`\nExpire date:(?P<expire_date>.*)\n`)
		params := m.FindStringSubmatch(certInfo)

		currentTimestamp := time.Now().Unix()
		//Linux: t, err := time.Parse("2006-1-2 15:04:05 MST", params[1])
		t, err := time.Parse("Jan 2 15:04:05 2006 MST", params[1])
		if err != nil {
			log.Println("E! failed to time: ", err)
			return result
		}

		expireTimestamp := t.Unix()
		expireSec := expireTimestamp - currentTimestamp
		if expireSec < 0 {
			result.SSLResult = 0
			result.SSLExpire = -1
			return result
		}
		result.SSLExpire = int(expireSec / 86400)

		if sslVerify == 0 {
			result.SSLResult = 1
			return result
		}
	}
	return result
}

func (ins *Instance) Gather(slist *types.SampleList) {

	if len(ins.Urls) == 0 {
		return
	}

	wg := new(sync.WaitGroup)
	for _, url := range ins.Urls {
		wg.Add(1)
		url := url
		go func(target string) {
			defer wg.Done()
			ins.gather(slist, url)
		}(url)
	}
	wg.Wait()
}

func (ins *Instance) gather(slist *types.SampleList, url string) {
	time.Sleep(time.Second * 1)

	if config.Config.DebugMode {
		log.Println("D! curl_response... url:", url)
	}

	labels := map[string]string{"url": url}
	fields := map[string]interface{}{}

	defer func() {
		slist.PushSamples(inputName, fields, labels)
	}()

	result := ins.DoCurl(url)

	fields["result"] = result.InfoResult

	if result.InfoResult != 1 {
		fields["total_time"] = -1
		fields["namelookup_time"] = -1
		fields["connect_time"] = -1
		fields["pertransfer_time"] = -1
		fields["starttransfer_time"] = -1
		fields["ssl_result"] = -1
		fields["ssl_expire"] = -1
	} else {
		fields["total_time"] = result.InfoTotalTime
		fields["namelookup_time"] = result.InfoNamelookupTime
		fields["connect_time"] = result.InfoConnectTime
		fields["pertransfer_time"] = result.InfoPretransferTime
		fields["starttransfer_time"] = result.InfoStarttransferTime
		if strings.Contains(url, "https://") {
			fields["ssl_result"] = result.SSLResult
			fields["ssl_expire"] = result.SSLExpire
		}
	}

	return

}
