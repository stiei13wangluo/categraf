package tencent_captcha

import (
	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/types"
	captcha "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/captcha/v20190722"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"log"
	"sync"
	"time"
)

const inputName = "tencent_captcha"

// Instance 配置文件
type Instance struct {
	config.InstanceConfig

	CaptchaAppId string `toml:"captcha_appid"`
	SecretId     string `toml:"secret_id"`
	SecretKey    string `toml:"secret_key"`
	StartTimeStr string `toml:"start_time_str"`
	EndTimeStr   string `toml:"end_time_str"`
	Dimension    string `toml:"dimension"`
}

// Init 初始化
func (ins *Instance) Init() error {

	now := time.Now()

	if ins.CaptchaAppId == "" {
		return types.ErrInstancesEmpty
	}

	if ins.SecretId == "" {
		return types.ErrInstancesEmpty
	}

	if ins.SecretKey == "" {
		return types.ErrInstancesEmpty
	}

	if ins.Dimension == "" {
		ins.Dimension = "3"
	}

	if ins.StartTimeStr == "" {
		ins.StartTimeStr = now.Format("2006-01-02") + " 00:00"

	}
	//
	if ins.EndTimeStr == "" {
		ins.EndTimeStr = now.Format("2006-01-02 15:04")
	}

	return nil
}

// TencentCaptcha 事例配置
type TencentCaptcha struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

func (tc *TencentCaptcha) Clone() inputs.Input {
	return &TencentCaptcha{}
}

func (tc *TencentCaptcha) Name() string {
	return inputName
}

func (tc *TencentCaptcha) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(tc.Instances))
	for i := 0; i < len(tc.Instances); i++ {
		ret[i] = tc.Instances[i]
	}
	return ret
}

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &TencentCaptcha{}
	})
}

func (ins *Instance) GetCaptchaStatus() (*captcha.GetTicketStatisticsResponse, error) {

	// Prepare fi elds and tags
	credential := common.NewCredential(
		ins.SecretId,
		ins.SecretKey,
	)

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "captcha.tencentcloudapi.com"
	// 实例化要请求产品的client对象,clientProfile是可选的
	client, _ := captcha.NewClient(credential, "", cpf)

	// 实例化一个请求对象,每个接口都会对应一个request对象
	request := captcha.NewGetTicketStatisticsRequest()

	request.CaptchaAppId = common.StringPtr(ins.CaptchaAppId)
	request.StartTimeStr = common.StringPtr(ins.StartTimeStr)
	request.EndTimeStr = common.StringPtr(ins.EndTimeStr)
	request.Dimension = common.StringPtr(ins.Dimension)

	// 返回的resp是一个GetTicketStatisticsResponse的实例，与请求对象对应
	response, err := client.GetTicketStatistics(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		//fmt.Printf("An API error has returned: %s", err)
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (ins *Instance) Gather(slist *types.SampleList) {

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ins.gather(slist)
	}()
	wg.Wait()
}

func (ins *Instance) gather(slist *types.SampleList) {
	if config.Config.DebugMode {
		log.Println("D! tencent_cloud_captcha... captcha_app_id:", ins.CaptchaAppId)
	}

	labels := map[string]string{"captcha_app_id": ins.CaptchaAppId}
	fields := map[string]interface{}{}

	defer func() {
		slist.PushSamples(inputName, fields, labels)
	}()

	status, err := ins.GetCaptchaStatus()

	if err != nil {
		fields["result"] = 0
		log.Println("E! failed to gather tencent_cloud_captcha captcha_app_id:", "error:", err)
		return
	}

	if *status.Response.CaptchaCode == 0 {
		fields["result"] = 1
	}

	if *status.Response.CaptchaCode != 0 {
		fields["result"] = 2
	}

	return

}
