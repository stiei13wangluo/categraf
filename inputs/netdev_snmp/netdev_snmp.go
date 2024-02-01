package netdev_snmp

import (
	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/pkg/runtimex"
	"flashcat.cloud/categraf/types"
	"github.com/gaochao1/sw"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/toolkits/pkg/concurrent/semaphore"
	"log"
	"strings"
	"sync"
	"time"
)

const inputName = "netdev_snmp"

type ChIfStat struct {
	IP          string
	UseTime     time.Duration
	IfStatsList []IfStats
}

type Custom struct {
	Metric string            `toml:"metric"`
	Tags   map[string]string `toml:"tags"`
	OID    string            `toml:"oid"`
}

type NetDev struct {
	config.PluginConfig
	Instances     []*Instance       `toml:"instances"`
	SwitchIdLabel string            `toml:"switch_id_label"`
	Mappings      map[string]string `toml:"mappings"`
}

type Instance struct {
	config.InstanceConfig

	IPs       []string `toml:"ips"`
	Community string   `toml:"community"`
	//IndexTag     bool     `toml:"index_tag"`
	IgnoreIfaces          []string `toml:"ignore_ifaces"`
	ConcurrencyForAddress int      `toml:"concurrency_for_address"`
	ConcurrencyForRequest int      `toml:"concurrency_for_request"`
	MaxRepetitions        int      `toml:"max_repetitions"`
	GetRepetitions        int      `toml:"get_repetitions"`
	FlowMetrics           bool     `toml:"flow_metrics"`
	CpuMetrics            bool     `toml:"cpu_metrics"`
	MemMetrics            bool     `toml:"mem_metrics"`
	ErrorsMetrics         bool     `toml:"errors_metrics"`
	DiscardsMetrics       bool     `toml:"discards_metrics"`
	BroadcastMetrics      bool     `toml:"broadcast_metrics"`
	MulticastMetrics      bool     `toml:"multicast_metrics"`
	OperStatusMetrics     bool     `toml:"oper_status_metrics"`
	SpeedMetrics          bool     `toml:"speed_metrics"`
	SnmpTimeoutSec        int      `toml:"snmp_timeout_sec"`
	SnmpRetries           int      `toml:"snmp_retries"`
	//SnmpModeGosnmp        bool     `toml:"snmp_mode_gosnmp"`

	Customs []Custom `toml:"customs"`

	parent *NetDev
}

type NetDevSnmp struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &NetDev{}
	})
}

func (ns *NetDev) Clone() inputs.Input {
	return &NetDev{}
}

func (ns *NetDev) Name() string {
	return inputName
}

func (ns *NetDev) MappingIP(ip string) string {
	val, has := ns.Mappings[ip]
	if has {
		return val
	}
	return ip
}

func (ns *NetDev) Init() error {
	if len(ns.Instances) == 0 {
		return types.ErrInstancesEmpty
	}

	for i := 0; i < len(ns.Instances); i++ {
		ns.Instances[i].parent = ns
	}

	return nil
}

func (ns *NetDev) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(ns.Instances))
	for i := 0; i < len(ns.Instances); i++ {
		ret[i] = ns.Instances[i]
	}
	return ret
}
func (ins *Instance) gatherMemMetrics(ips []string, slist *types.SampleList) {
	result := cmap.New()
	for i := 0; i < len(ips); i++ {
		result.Set(ips[i], -1.0)
	}

	wg := new(sync.WaitGroup)
	se := semaphore.NewSemaphore(ins.ConcurrencyForAddress)
	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		wg.Add(1)
		se.Acquire()
		go ins.memstat(wg, se, ip, result)
	}
	wg.Wait()

	for ip, utilPercentInterface := range result.Items() {
		utilPercent, ok := utilPercentInterface.(float64)
		if !ok {
			continue
		}

		if utilPercent < 0 {
			continue
		}

		slist.PushFront(types.NewSample(inputName, "mem_util", utilPercent, map[string]string{ins.parent.SwitchIdLabel: ins.parent.MappingIP(ip)}))
	}
}

func (ins *Instance) memstat(wg *sync.WaitGroup, sema *semaphore.Semaphore, ip string, result cmap.ConcurrentMap) {
	defer func() {
		sema.Release()
		wg.Done()
	}()

	utilPercent, err := sw.MemUtilization(ip, ins.Community, ins.SnmpTimeoutSec*1000, ins.SnmpRetries)
	if err != nil {
		log.Println("E! failed to gather mem, ip:", ip, "error:", err)
		return
	}

	result.Set(ip, float64(utilPercent))
}

func (ins *Instance) gatherCpuMetrics(ips []string, slist *types.SampleList) {
	result := cmap.New()
	for i := 0; i < len(ips); i++ {
		result.Set(ips[i], -1.0)
	}

	wg := new(sync.WaitGroup)
	se := semaphore.NewSemaphore(ins.ConcurrencyForAddress)
	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		wg.Add(1)
		se.Acquire()
		go ins.cpustat(wg, se, ip, result)
	}
	wg.Wait()

	for ip, utilPercentInterface := range result.Items() {
		utilPercent, ok := utilPercentInterface.(float64)
		if !ok {
			continue
		}

		if utilPercent < 0 {
			continue
		}

		slist.PushFront(types.NewSample(inputName, "cpu_util", utilPercent, map[string]string{ins.parent.SwitchIdLabel: ins.parent.MappingIP(ip)}))
	}
}

func (ins *Instance) cpustat(wg *sync.WaitGroup, sema *semaphore.Semaphore, ip string, result cmap.ConcurrentMap) {
	defer func() {
		sema.Release()
		wg.Done()
	}()

	utilPercent, err := sw.CpuUtilization(ip, ins.Community, ins.SnmpTimeoutSec*1000, ins.SnmpRetries)
	if err != nil {
		log.Println("E! failed to gather cpu, ip:", ip, "error:", err)
		return
	}

	result.Set(ip, float64(utilPercent))
}

func (ins *Instance) gatherFlowMetrics(ips []string, slist *types.SampleList) {

	result := cmap.New()
	for i := 0; i < len(ips); i++ {
		result.Set(ips[i], nil)
	}
	wg := new(sync.WaitGroup)
	se := semaphore.NewSemaphore(ins.ConcurrencyForAddress)
	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		wg.Add(1)
		se.Acquire()
		go ins.ifstat(wg, se, ip, result)
	}
	wg.Wait()

	for ip, chifstatInterface := range result.Items() {
		if chifstatInterface == nil {
			continue
		}

		chifstat, ok := chifstatInterface.(*ChIfStat)
		//slist.PushFront(types.NewSample(inputName, "ifstat_use_time_sec", chifstat.UseTime.Seconds(), map[string]string{ins.parent.SwitchIdLabel: ins.parent.MappingIP(ip)}))
		if !ok {
			continue
		}

		if chifstat == nil {
			continue
		}

		if chifstat.IP == "" {
			continue
		}

		if ins.FlowMetrics {
			stats := chifstat.IfStatsList
			for i := 0; i < len(stats); i++ {

				//fmt.Printf("%v:%v:in:%v:%v\n", i, ins.parent.MappingIP(ip), stats[i].IfName, stats[i].IfHCInOctets)
				//fmt.Printf("%v:%v:out:%v:%v\n", i, ins.parent.MappingIP(ip), stats[i].IfName, stats[i].IfHCOutOctets)

				ifStat := stats[i]

				tags := map[string]string{
					ins.parent.SwitchIdLabel: ins.parent.MappingIP(ip),
					"ifname":                 ifStat.IfName,
				}

				if ifStat.ifAlias == "" {
					tags["ifalias"] = ifStat.IfName
				} else {
					tags["ifalias"] = ifStat.ifAlias
				}

				fields := make(map[string]map[string]interface{})

				fields["IfHCInOctetsMap"] = map[string]interface{}{
					"IfHCInOctets": ifStat.IfHCInOctets,
				}

				fields["IfHCOutOctetsMap"] = map[string]interface{}{
					"IfHCOutOctets": ifStat.IfHCOutOctets,
				}

				//IfHCInOctetsMap := make(map[string]interface{})
				//IfHCOutOctetsMap := make(map[string]interface{})
				//
				//fields[]

				//fields := make(map[string]int, 10)

				//fields["IfHCInOctets"] = map[string]interface{}{""}

				//fields["IfHCInOctets"] = ifStat.IfHCInOctets
				slist.PushSamples(inputName, fields["IfHCInOctetsMap"], tags)
				slist.PushSamples(inputName, fields["IfHCOutOctetsMap"], tags)

				//slist.PushFront(types.NewSample(inputName, "IfHCInOctets", ifStat.IfHCInOctets, tags))
				//slist.PushFront(types.NewSample(inputName, "IfHCOutOctets", ifStat.IfHCOutOctets, tags))
				if ins.OperStatusMetrics {
					fields["IfOperStatusMap"] = map[string]interface{}{
						"IfOperStatus": ifStat.IfOperStatus,
					}
					slist.PushSamples(inputName, fields["IfOperStatusMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfOperStatus", ifStat.IfOperStatus, tags))
				}
				if ins.SpeedMetrics {
					fields["IfHighSpeedMap"] = map[string]interface{}{
						"IfHighSpeed": ifStat.IfHighSpeed,
					}
					slist.PushSamples(inputName, fields["IfHighSpeedMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfHighSpeed", ifStat.IfHighSpeed, tags))
				}
				if ins.ErrorsMetrics {
					fields["IfInErrorsMap"] = map[string]interface{}{
						"IfInErrors": ifStat.IfInErrors,
					}
					fields["IfOutErrorsMap"] = map[string]interface{}{
						"IfOutErrors": ifStat.IfOutErrors,
					}
					slist.PushSamples(inputName, fields["IfInErrorsMap"], tags)
					slist.PushSamples(inputName, fields["IfOutErrorsMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfInErrors", ifStat.IfInErrors, tags))
					//slist.PushFront(types.NewSample(inputName, "IfOutErrors", ifStat.IfOutErrors, tags))
				}
				if ins.DiscardsMetrics {
					fields["IfInDiscardsMap"] = map[string]interface{}{
						"IfInDiscards": ifStat.IfInDiscards,
					}
					fields["IfOutDiscardsMap"] = map[string]interface{}{
						"IfOutDiscards": ifStat.IfOutDiscards,
					}
					slist.PushSamples(inputName, fields["IfInDiscardsMap"], tags)
					slist.PushSamples(inputName, fields["IfOutDiscardsMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfInDiscards", ifStat.IfInDiscards, tags))
					//slist.PushFront(types.NewSample(inputName, "IfOutDiscards", ifStat.IfOutDiscards, tags))
				}
				if ins.MulticastMetrics {
					fields["IfHCInMulticastPktsMap"] = map[string]interface{}{
						"IfHCInMulticastPkts": ifStat.IfHCInMulticastPkts,
					}
					fields["IfHCOutMulticastPktsMap"] = map[string]interface{}{
						"IfHCOutMulticastPkts": ifStat.IfHCOutMulticastPkts,
					}
					slist.PushSamples(inputName, fields["IfHCInMulticastPktsMap"], tags)
					slist.PushSamples(inputName, fields["IfHCOutMulticastPktsMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfHCInMulticastPkts", ifStat.IfHCInMulticastPkts, tags))
					//slist.PushFront(types.NewSample(inputName, "IfHCOutMulticastPkts", ifStat.IfHCOutMulticastPkts, tags))
				}
				if ins.BroadcastMetrics {
					fields["IfHCInBroadcastPktsMap"] = map[string]interface{}{
						"IfHCInBroadcastPkts": ifStat.IfHCInBroadcastPkts,
					}
					fields["IfHCOutBroadcastPktsMap"] = map[string]interface{}{
						"IfHCOutBroadcastPkts": ifStat.IfHCOutBroadcastPkts,
					}
					slist.PushSamples(inputName, fields["IfHCInBroadcastPktsMap"], tags)
					slist.PushSamples(inputName, fields["IfHCOutBroadcastPktsMap"], tags)
					//slist.PushFront(types.NewSample(inputName, "IfHCInBroadcastPkts", ifStat.IfHCInBroadcastPkts, tags))
					//slist.PushFront(types.NewSample(inputName, "IfHCOutBroadcastPkts", ifStat.IfHCOutBroadcastPkts, tags))
				}
			}

		}

	}

}

func (ins *Instance) custstat(wg *sync.WaitGroup, ip string, slist *types.SampleList, cust []Custom) {
	defer wg.Done()

	defer func() {
		if r := recover(); r != nil {
			log.Println("E! recovered in custstat, ip:", ip, "oid:", cust, "error:", r, "stack:", runtimex.Stack(3))
		}
	}()

	metricMap := map[string]string{}
	tagsMap := map[string]map[string]string{}

	var oidsList []string

	for i := 0; i < len(cust); i++ {
		oid := cust[i].OID
		if !strings.HasPrefix(cust[i].OID, ".") {
			oid = "." + cust[i].OID
		}
		metricMap[oid] = cust[i].Metric
		tagsMap[oid] = cust[i].Tags
		oidsList = append(oidsList, oid)
	}

	snmpValueList, err := CustomSnmpValue(ip, oidsList, ins.Community, ins.SnmpTimeoutSec, ins.SnmpRetries, ins.ConcurrencyForRequest)

	if err == nil {
		for k := range snmpValueList {
			slist.PushFront(types.NewSample(inputName, metricMap[k], snmpValueList[k], tagsMap[k], map[string]string{ins.parent.SwitchIdLabel: ins.parent.MappingIP(ip)}))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (ins *Instance) gatherCustoms(ips []string, slist *types.SampleList) {
	wg := new(sync.WaitGroup)

	for i := 0; i < len(ips); i++ {
		ip := ips[i]
		wg.Add(1)
		go ins.custstat(wg, ip, slist, ins.Customs)
	}

	wg.Wait()
}

func (ins *Instance) ifstat(wg *sync.WaitGroup, sema *semaphore.Semaphore, ip string, result cmap.ConcurrentMap) {
	defer func() {
		sema.Release()
		wg.Done()
	}()

	var (
		ifList []IfStats
		err    error
		start  = time.Now()
	)

	ifList, err = ListIfStats(ip, ins.Community, ins.SnmpTimeoutSec, ins.IgnoreIfaces, ins.SnmpRetries,
		ins.ConcurrencyForRequest, ins.MaxRepetitions, ins.GetRepetitions, ins.OperStatusMetrics, ins.SpeedMetrics,
		ins.ErrorsMetrics, ins.DiscardsMetrics, ins.BroadcastMetrics, ins.MulticastMetrics,
	)

	if config.Config.DebugMode {
		log.Println("D! switch gather ifstat, ip:", ip, "use:", time.Since(start))
	}

	if err != nil {
		log.Println("E! failed to gather ifstat, ip:", ip, "error:", err)
		return
	}

	if len(ifList) > 0 {
		result.Set(ip, &ChIfStat{
			IP:          ip,
			IfStatsList: ifList,
			UseTime:     time.Since(start),
		})
	}
}

func (ins *Instance) Gather(slist *types.SampleList) {

	start := time.Now()
	defer func() {
		//for i := 0; i < len(ins.IPs); i++ {
		//	go Work(ins.IPs[i])
		//}
		log.Println("I! netdev_snmp gather use:", time.Since(start))
	}()

	if ins.FlowMetrics {
		ins.gatherFlowMetrics(ins.IPs, slist)
	}

	if ins.CpuMetrics {
		ins.gatherCpuMetrics(ins.IPs, slist)
	}

	if ins.MemMetrics {
		ins.gatherMemMetrics(ins.IPs, slist)
	}

	if len(ins.Customs) > 0 {
		ins.gatherCustoms(ins.IPs, slist)
	}

}
