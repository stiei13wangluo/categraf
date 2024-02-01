package powerdns_recursor

import (
	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/types"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"
)

const inputName = "powerdns_recursor"

var defaultTimeout = 5 * time.Second

type Instance struct {
	UnixSockets            []string `toml:"unix_sockets"`
	SocketDir              string   `toml:"socket_dir"`
	SocketMode             string   `toml:"socket_mode"`
	ControlProtocolVersion int      `toml:"control_protocol_version"`
	mode                   uint32
	gatherFromServer       func(address string, slist *types.SampleList) error
	config.InstanceConfig
}

type PowerdnsRecursor struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &PowerdnsRecursor{}
	})
}

func (pr *PowerdnsRecursor) Clone() inputs.Input {
	return &PowerdnsRecursor{}
}

func (pr *PowerdnsRecursor) Name() string {
	return inputName
}

func (pr *PowerdnsRecursor) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(pr.Instances))
	for i := 0; i < len(pr.Instances); i++ {
		ret[i] = pr.Instances[i]
	}
	return ret
}

func (ins *Instance) Init() error {
	if ins.SocketMode != "" {
		mode, err := strconv.ParseUint(ins.SocketMode, 8, 32)
		if err != nil {
			return fmt.Errorf("could not parse socket_mode: %w", err)
		}

		ins.mode = uint32(mode)
	}

	if ins.SocketDir == "" {
		ins.SocketDir = filepath.Join("/", "var", "run")
	}

	switch ins.ControlProtocolVersion {
	// We treat 0 the same as 1 since it's the default value if a user doesn't explicitly specify one.
	case 0, 1:
		ins.gatherFromServer = ins.gatherFromV1Server
	case 2:
		ins.gatherFromServer = ins.gatherFromV2Server
	case 3:
		ins.gatherFromServer = ins.gatherFromV3Server
	default:
		return fmt.Errorf("unknown control protocol version '%d', allowed values are 1, 2, 3", ins.ControlProtocolVersion)
	}

	if len(ins.UnixSockets) == 0 {
		ins.UnixSockets = []string{"/var/run/pdns_recursor.controlsocket"}
	}

	return nil
}

func (ins *Instance) Gather(slist *types.SampleList) {

	for _, serverSocket := range ins.UnixSockets {
		if err := ins.gatherFromServer(serverSocket, slist); err != nil {
			log.Println("E!", err)
		}
	}
}
