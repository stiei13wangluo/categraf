package netdev_snmp

import (
	g "github.com/gosnmp/gosnmp"
	"log"
	"strconv"
	"strings"
	"time"
)

func RunBulkWalk(ip string, oid string, community string, timeout int, retry int, maxRepetitions int) (result []g.SnmpPDU, err error) {
	curGosnmp := &g.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   g.Version2c,
		Timeout:   time.Duration(timeout) * time.Second,
		Retries:   retry,
		//Logger:         g.NewLogger(log.New(os.Stdout, "", 0)),
		MaxRepetitions: uint32(maxRepetitions),
	}

	err = curGosnmp.Connect()

	if err != nil {
		return
	}

	defer curGosnmp.Conn.Close()

	result, err = curGosnmp.BulkWalkAll(oid)

	if err != nil {
		return
	}

	return
}

func RunSnmp(ip string, oids []string, community string, timeout int, retry int) (result *g.SnmpPacket, err error) {

	curGosnmp := &g.GoSNMP{
		Target:    ip,
		Port:      uint16(161),
		Community: community,
		Version:   g.Version2c,
		Timeout:   time.Duration(timeout) * time.Second,
		Retries:   retry,
		//Logger:         g.NewLogger(log.New(os.Stdout, "", 0)),
		//MaxRepetitions: 10,
	}

	err = curGosnmp.Connect()

	if err != nil {
		return
	}
	defer curGosnmp.Conn.Close()

	result, err = curGosnmp.Get(oids)
	if err != nil {
		return
	}

	return
}

func GetStrSnmpValue(ip string, oids []string, community string, timeout int, retry int) ([]string, error) {
	var result *g.SnmpPacket
	var err error
	var snmpValue []string
	result, err = RunSnmp(ip, oids, community, timeout, retry)

	if err != nil {
		log.Fatalf("Get() err: %v", err)
		return nil, err
	}

	for _, variable := range result.Variables {
		snmpValue = append(snmpValue, string(variable.Value.([]byte)))
	}

	return snmpValue, nil
}

func GetInt64SnmpValue(ip string, oids []string, community string, timeout int, retry int) ([]int64, error) {
	var result *g.SnmpPacket
	var err error
	var snmpValue []int64

	result, err = RunSnmp(ip, oids, community, timeout, retry)

	if err != nil {
		log.Fatalf("Get() err: %v", err)
		return nil, err
	}

	for _, variable := range result.Variables {

		if variable.Value == "NoSuchInstance" {
			continue
		}
		snmpValue = append(snmpValue, g.ToBigInt(variable.Value).Int64())
	}

	return snmpValue, nil
}

func CustomSnmpValue(ip string, oids []string, community string, timeout int, retry int, get_req_sum int) (map[string]int64, error) {
	var snmpPacket *g.SnmpPacket
	var err error

	ValueListMap := map[string]int64{}
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
			return nil, err
		}

		for _, variable := range snmpPacket.Variables {
			var SnmpValue int64
			switch variable.Type {
			case g.NoSuchInstance:
				continue
			case g.OctetString:
				Value := string(variable.Value.([]byte))
				if len(Value) == 0 {
					SnmpValue = int64(-4000)
				} else if strings.Contains(strconv.Quote(Value), "\\x") {
					Value = strings.Trim(Value, "\\x00")
					fmtValue, err := strconv.Atoi(Value)
					if err != nil {
						return nil, err
					}
					SnmpValue = int64(fmtValue)
				} else {
					SnmpValue = g.ToBigInt(strconv.Itoa(findStrMin(splitString(Value, ",")))).Int64()
				}
			default:
				SnmpValue = g.ToBigInt(variable.Value).Int64()
			}
			ValueListMap[variable.Name] = SnmpValue
		}

		time.Sleep(5 * time.Millisecond)

	}
	return ValueListMap, nil
}
