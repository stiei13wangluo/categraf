package netdev_snmp

import (
	"fmt"
	"log"
	"strings"
	"time"

	g "github.com/gosnmp/gosnmp"
)

const (
	sysDescrOid                   = ".1.3.6.1.2.1.1.1.0"
	ifNameOid                     = "1.3.6.1.2.1.31.1.1.1.1"
	ifAliasOidPrefix              = ".1.3.6.1.2.1.31.1.1.1.18."
	ifNameOidPrefix               = ".1.3.6.1.2.1.31.1.1.1.1."
	ifHCInOid                     = "1.3.6.1.2.1.31.1.1.1.6"
	ifHCInOidPrefix               = ".1.3.6.1.2.1.31.1.1.1.6."
	ifHCOutOid                    = "1.3.6.1.2.1.31.1.1.1.10"
	ifHCOutOidPrefix              = ".1.3.6.1.2.1.31.1.1.1.10."
	ifHCInPktsOid                 = "1.3.6.1.2.1.31.1.1.1.7"
	ifHCInPktsOidPrefix           = ".1.3.6.1.2.1.31.1.1.1.7."
	ifHCOutPktsOid                = "1.3.6.1.2.1.31.1.1.1.11"
	ifOperStatusOid               = "1.3.6.1.2.1.2.2.1.8"
	ifOperStatusOidPrefix         = ".1.3.6.1.2.1.2.2.1.8."
	ifHCInBroadcastPktsOid        = "1.3.6.1.2.1.31.1.1.1.9"
	ifHCInBroadcastPktsOidPrefix  = ".1.3.6.1.2.1.31.1.1.1.9."
	ifHCOutBroadcastPktsOid       = "1.3.6.1.2.1.31.1.1.1.13"
	ifHCOutBroadcastPktsOidPrefix = ".1.3.6.1.2.1.31.1.1.1.13."
	// multicastpkt
	ifHCInMulticastPktsOid        = "1.3.6.1.2.1.31.1.1.1.8"
	ifHCInMulticastPktsOidPrefix  = ".1.3.6.1.2.1.31.1.1.1.8."
	ifHCOutMulticastPktsOid       = "1.3.6.1.2.1.31.1.1.1.12"
	ifHCOutMulticastPktsOidPrefix = ".1.3.6.1.2.1.31.1.1.1.12."
	// speed 配置
	ifSpeedOid           = "1.3.6.1.2.1.31.1.1.1.15"
	ifHighSpeedOidPrefix = ".1.3.6.1.2.1.31.1.1.1.15."

	// Discards配置
	ifInDiscardsOid       = "1.3.6.1.2.1.2.2.1.13"
	ifInDiscardsOidPrefix = ".1.3.6.1.2.1.2.2.1.13."
	ifOutDiscardsOid      = "1.3.6.1.2.1.2.2.1.19"

	// Errors配置
	ifInErrorsOid        = "1.3.6.1.2.1.2.2.1.14"
	ifInErrorsOidPrefix  = ".1.3.6.1.2.1.2.2.1.14."
	ifOutErrorsOid       = "1.3.6.1.2.1.2.2.1.20"
	ifOutErrorsOidPrefix = ".1.3.6.1.2.1.2.2.1.20."

	//ifInUnknownProtos 由于未知或不支持的网络协议而丢弃的输入报文的数量
	ifInUnknownProtosOid    = "1.3.6.1.2.1.2.2.1.15"
	ifInUnknownProtosPrefix = ".1.3.6.1.2.1.2.2.1.15."

	//ifOutQLen 接口上输出报文队列长度
	ifOutQLenOid    = "1.3.6.1.2.1.2.2.1.21"
	ifOutQLenPrefix = ".1.3.6.1.2.1.2.2.1.21."
)

type IfStats struct {
	IfName  string
	ifAlias string
	//IfIndex              int
	IfHCInOctets         uint64
	IfHCOutOctets        uint64
	IfHCInBroadcastPkts  uint64
	IfHCOutBroadcastPkts uint64
	IfHCInMulticastPkts  uint64
	IfHCOutMulticastPkts uint64
	IfHighSpeed          int
	IfInDiscards         uint64
	IfOutDiscards        uint64
	IfInErrors           uint64
	IfOutErrors          uint64
	IfOperStatus         int
	//TS           int64
}

type IfStatsOid struct {
	IfAlias              []string
	IfHCInOctets         []string
	IfHCOutOctets        []string
	IfHighSpeed          []string
	IfOperStatus         []string
	IfInDiscards         []string
	IfOutDiscards        []string
	IfInErrors           []string
	IfOutErrors          []string
	IfHCInBroadcastPkts  []string
	IfHCOutBroadcastPkts []string
	IfHCInMulticastPkts  []string
	IfHCOutMulticastPkts []string
}

func WalkIfName(ip string, oid string, community string, ignoreIface []string, timeout int, retry int, max_repetitions int, limitCh chan bool, ch chan []string, ch2 chan map[string]string) {
	var snmpPDUs []g.SnmpPDU
	var err error
	ifNameMap := map[string]string{}
	var ifIndexList []string

	snmpPDUs, err = RunBulkWalk(ip, oid, community, timeout, retry, max_repetitions)
	if err != nil {
		log.Println(ip, oid, err)
		close(ch)
		close(ch2)
		<-limitCh
		return
	}

	for i := 0; i < len(snmpPDUs); i++ {
		checkIf := false
		ifName := fmt.Sprintf("%v", string(snmpPDUs[i].Value.([]byte)))
		if len(ignoreIface) > 0 {
			for _, ignore := range ignoreIface {
				if strings.Contains(ifName, ignore) {
					checkIf = true
					break
				}
			}
		}

		if checkIf {
			continue
		}

		oidIndex := getOidIndex(snmpPDUs[i].Name)
		ifNameMap[oidIndex] = ifName
		ifIndexList = append(ifIndexList, oidIndex)

	}
	ch <- ifIndexList
	ch2 <- ifNameMap
	<-limitCh
	return
}

func WalkIf(ip string, oids []string, community string, timeout int, retry int, get_req_sum int, limitCh chan bool, ch chan map[string]int64) {

	var snmpPacket *g.SnmpPacket
	var err error

	ifMap := map[string]int64{}
	oidLen := len(oids)
	reLen := oidLen / get_req_sum

	for i := 0; i < reLen+1; i++ {
		num1 := i * get_req_sum
		num2 := (i + 1) * get_req_sum

		if (i+1)*get_req_sum > oidLen {
			num2 = num1 + (oidLen - i*get_req_sum)
		}

		snmpPacket, err = RunSnmp(ip, oids[num1:num2], community, timeout, retry)
		if err != nil {
			close(ch)
			<-limitCh
			return
		}

		for _, variable := range snmpPacket.Variables {
			oidIndex := getOidIndex(variable.Name)
			ifMap[oidIndex] = g.ToBigInt(variable.Value).Int64()
		}
		time.Sleep(5 * time.Millisecond)

	}

	ch <- ifMap
	<-limitCh
	return
}

func WalkIfAlias(ip string, oids []string, community string, timeout int, retry int, get_req_sum int, limitCh chan bool, ch chan map[string]string) {

	var snmpPacket *g.SnmpPacket
	var err error

	ifMap := map[string]string{}
	oidLen := len(oids)
	reLen := oidLen / get_req_sum

	for i := 0; i < reLen+1; i++ {
		num1 := i * get_req_sum
		num2 := (i + 1) * get_req_sum

		if (i+1)*get_req_sum > oidLen {
			num2 = num1 + (oidLen - i*get_req_sum)
		}

		snmpPacket, err = RunSnmp(ip, oids[num1:num2], community, timeout, retry)
		if err != nil {
			close(ch)
			<-limitCh
			return
		}

		for _, variable := range snmpPacket.Variables {
			oidIndex := getOidIndex(variable.Name)
			ifMap[oidIndex] = string(variable.Value.([]byte))
		}
		time.Sleep(5 * time.Millisecond)

	}

	ch <- ifMap
	<-limitCh
	return
}

func ListIfStats(ip string, community string, timeout int, ignoreIface []string, retry int, limitConn int,
	max_repetitions int, get_req_sum int, oper_status_metrics bool, speed_metrics bool, errors_metrics bool, discards_metrics bool,
	broadcast_metrics bool, multicast_metrics bool) ([]IfStats, error) {

	var limitCh chan bool
	if limitConn > 0 {
		limitCh = make(chan bool, limitConn)
	} else {
		limitCh = make(chan bool, 1)
	}

	//limitCh := make(chan bool, 1)

	var ifStatsList []IfStats

	defer func() {
		if r := recover(); r != nil {
			log.Println(ip+" Recovered in ListIfStats", r)
		}
	}()

	chifIndexList := make(chan []string)
	chifNameMap := make(chan map[string]string)

	limitCh <- true
	go WalkIfName(ip, ifNameOid, community, ignoreIface, timeout, retry, max_repetitions, limitCh, chifIndexList, chifNameMap)
	time.Sleep(5 * time.Millisecond)

	ifIndexList := <-chifIndexList
	ifNameMap := <-chifNameMap

	if len(ifIndexList) == 0 || len(ifNameMap) == 0 {
		return ifStatsList, nil
	}

	var ifStatsOid IfStatsOid

	for i := 0; i < len(ifIndexList); i++ {
		ifStatsOid.IfAlias = append(ifStatsOid.IfAlias, ifAliasOidPrefix+ifIndexList[i])
		ifStatsOid.IfHCInOctets = append(ifStatsOid.IfHCInOctets, ifHCInOidPrefix+ifIndexList[i])
		ifStatsOid.IfHCOutOctets = append(ifStatsOid.IfHCOutOctets, ifHCOutOidPrefix+ifIndexList[i])
		ifStatsOid.IfHighSpeed = append(ifStatsOid.IfHighSpeed, ifHighSpeedOidPrefix+ifIndexList[i])
		ifStatsOid.IfOperStatus = append(ifStatsOid.IfOperStatus, ifOperStatusOidPrefix+ifIndexList[i])
		ifStatsOid.IfInErrors = append(ifStatsOid.IfInErrors, ifInErrorsOidPrefix+ifIndexList[i])
		ifStatsOid.IfOutErrors = append(ifStatsOid.IfOutErrors, ifOutErrorsOidPrefix+ifIndexList[i])
		ifStatsOid.IfInDiscards = append(ifStatsOid.IfInDiscards, ifInDiscardsOid+ifIndexList[i])
		ifStatsOid.IfOutDiscards = append(ifStatsOid.IfOutDiscards, ifOutDiscardsOid+ifIndexList[i])
		ifStatsOid.IfHCInBroadcastPkts = append(ifStatsOid.IfHCInBroadcastPkts, ifHCInBroadcastPktsOidPrefix+ifIndexList[i])
		ifStatsOid.IfHCOutBroadcastPkts = append(ifStatsOid.IfHCOutBroadcastPkts, ifHCOutBroadcastPktsOidPrefix+ifIndexList[i])
		ifStatsOid.IfHCInMulticastPkts = append(ifStatsOid.IfHCInMulticastPkts, ifHCInMulticastPktsOidPrefix+ifIndexList[i])
		ifStatsOid.IfHCOutMulticastPkts = append(ifStatsOid.IfHCOutMulticastPkts, ifHCOutMulticastPktsOidPrefix+ifIndexList[i])
	}

	chifIn := make(chan map[string]int64)
	chifOut := make(chan map[string]int64)
	chifAlias := make(chan map[string]string)

	limitCh <- true
	go WalkIfAlias(ip, ifStatsOid.IfAlias, community, timeout, retry, get_req_sum, limitCh, chifAlias)
	time.Sleep(5 * time.Millisecond)
	ifAliasMap := <-chifAlias

	limitCh <- true
	go WalkIf(ip, ifStatsOid.IfHCInOctets, community, timeout, retry, get_req_sum, limitCh, chifIn)
	time.Sleep(5 * time.Millisecond)
	ifInMap := <-chifIn

	limitCh <- true
	go WalkIf(ip, ifStatsOid.IfHCOutOctets, community, timeout, retry, get_req_sum, limitCh, chifOut)
	time.Sleep(5 * time.Millisecond)
	ifOutMap := <-chifOut

	ifOperStatusMap := map[string]int64{}
	ifHighSpeedMap := map[string]int64{}
	ifInErrorsMap := map[string]int64{}
	ifOutErrorsMap := map[string]int64{}
	ifInDiscardsMap := map[string]int64{}
	ifOutDiscardsMap := map[string]int64{}
	ifHCInBroadcastPktsMap := map[string]int64{}
	ifHCOutBroadcastPktsMap := map[string]int64{}
	ifHCInMulticastPktsMap := map[string]int64{}
	ifHCOutMulticastPktsMap := map[string]int64{}

	if oper_status_metrics {
		chifOperStatus := make(chan map[string]int64)
		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfOperStatus, community, timeout, retry, get_req_sum, limitCh, chifOperStatus)
		time.Sleep(5 * time.Millisecond)
		ifOperStatusMap = <-chifOperStatus
	}

	if speed_metrics {
		chifHighSpeed := make(chan map[string]int64)
		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfHighSpeed, community, timeout, retry, get_req_sum, limitCh, chifHighSpeed)
		time.Sleep(5 * time.Millisecond)
		ifHighSpeedMap = <-chifHighSpeed
	}

	if errors_metrics {
		chifInErrors := make(chan map[string]int64)
		chifOutErrors := make(chan map[string]int64)

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfInErrors, community, timeout, retry, get_req_sum, limitCh, chifInErrors)
		time.Sleep(5 * time.Millisecond)
		ifInErrorsMap = <-chifInErrors

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfOutErrors, community, timeout, retry, get_req_sum, limitCh, chifOutErrors)
		time.Sleep(5 * time.Millisecond)
		ifOutErrorsMap = <-chifOutErrors

	}

	if discards_metrics {
		chifInDiscards := make(chan map[string]int64)
		chifOutDiscards := make(chan map[string]int64)

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfInDiscards, community, timeout, retry, get_req_sum, limitCh, chifInDiscards)
		time.Sleep(5 * time.Millisecond)
		ifInDiscardsMap = <-chifInDiscards

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfOutDiscards, community, timeout, retry, get_req_sum, limitCh, chifOutDiscards)
		time.Sleep(5 * time.Millisecond)
		ifOutDiscardsMap = <-chifOutDiscards
	}

	if broadcast_metrics {
		chifInBroadcastPkts := make(chan map[string]int64)
		chifOutBroadcastPkts := make(chan map[string]int64)

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfHCInBroadcastPkts, community, timeout, retry, get_req_sum, limitCh, chifInBroadcastPkts)
		time.Sleep(5 * time.Millisecond)
		ifHCInBroadcastPktsMap = <-chifInBroadcastPkts

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfHCOutBroadcastPkts, community, timeout, retry, get_req_sum, limitCh, chifOutBroadcastPkts)
		time.Sleep(5 * time.Millisecond)
		ifHCOutBroadcastPktsMap = <-chifOutBroadcastPkts
	}

	if multicast_metrics {
		chifInMulticastPkts := make(chan map[string]int64)
		chifOutMulticastPkts := make(chan map[string]int64)
		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfHCInMulticastPkts, community, timeout, retry, get_req_sum, limitCh, chifInMulticastPkts)
		time.Sleep(5 * time.Millisecond)
		ifHCInMulticastPktsMap = <-chifInMulticastPkts

		limitCh <- true
		go WalkIf(ip, ifStatsOid.IfHCOutMulticastPkts, community, timeout, retry, get_req_sum, limitCh, chifOutMulticastPkts)
		time.Sleep(5 * time.Millisecond)
		ifHCOutMulticastPktsMap = <-chifOutMulticastPkts
	}

	for i := 0; i < len(ifIndexList); i++ {
		var ifstat IfStats
		ifstat.IfName = ifNameMap[ifIndexList[i]]
		ifstat.ifAlias = ifAliasMap[ifIndexList[i]]
		ifstat.IfHCInOctets = uint64(ifInMap[ifIndexList[i]])
		ifstat.IfHCOutOctets = uint64(ifOutMap[ifIndexList[i]])

		if oper_status_metrics {
			if len(ifOperStatusMap) > 0 {
				ifstat.IfOperStatus = int(ifOperStatusMap[ifIndexList[i]])
			}
		}
		if speed_metrics {
			if len(ifHighSpeedMap) > 0 {
				ifstat.IfHighSpeed = int(ifHighSpeedMap[ifIndexList[i]])
			}
		}
		if errors_metrics {
			if len(ifInErrorsMap) > 0 && len(ifOutErrorsMap) > 0 {
				ifstat.IfInErrors = uint64(ifInErrorsMap[ifIndexList[i]])
				ifstat.IfOutErrors = uint64(ifOutErrorsMap[ifIndexList[i]])
			}
		}
		if discards_metrics {
			if len(ifInDiscardsMap) > 0 && len(ifOutDiscardsMap) > 0 {
				ifstat.IfInDiscards = uint64(ifInDiscardsMap[ifIndexList[i]])
				ifstat.IfOutDiscards = uint64(ifOutDiscardsMap[ifIndexList[i]])
			}
		}
		if multicast_metrics {
			if len(ifHCInMulticastPktsMap) > 0 && len(ifHCOutMulticastPktsMap) > 0 {
				ifstat.IfHCInMulticastPkts = uint64(ifHCInMulticastPktsMap[ifIndexList[i]])
				ifstat.IfHCOutMulticastPkts = uint64(ifHCOutMulticastPktsMap[ifIndexList[i]])
			}
		}
		if broadcast_metrics {
			if len(ifHCInBroadcastPktsMap) > 0 && len(ifHCOutBroadcastPktsMap) > 0 {
				ifstat.IfHCInBroadcastPkts = uint64(ifHCInBroadcastPktsMap[ifIndexList[i]])
				ifstat.IfHCOutBroadcastPkts = uint64(ifHCOutBroadcastPktsMap[ifIndexList[i]])
			}
		}
		ifStatsList = append(ifStatsList, ifstat)
	}

	return ifStatsList, nil
}
