package args

import (
	"flag"
	"github.com/pkg/errors"
)

// Config represents commandline arguments.
type Config struct {
	// [TEST MODE] enforces the following behaviours:
	// - `cxoUseMemory = true` (disables modification to cxo database).
	// - `configDir = "/tmp"` (disables modification to config files).
	testMode            bool // Whether to enable test mode.
	testModeThreads     int  // Number of threads to use for test mode (will create them in test mode).
	testModeMinInterval int  // Minimum interval between simulated activity (in seconds).
	testModeMaxInterval int  // Maximum interval between simulated activity (in seconds).

	master            bool   // Whether BBS node can host boards.
	configDir         string // Configuration directory.
	rpcServerPort     int    // RPC server port (master node only).
	rpcServerRemAdr   string // RPC remote address (master node only).
	cxoPort           int    // Port of CXO Daemon.
	cxoUseMemory      bool   // Whether to use in-memory database for CXO.
	cxoDir            string // Folder name to store db.
	webGUIEnable      bool   // Whether to enable web GUI.
	webGUIPort        int    // Port of web GUI.
	webGUIOpenBrowser bool   // Whether to open browser on web GUI start.
}

// NewConfig makes Config with default values.
func NewConfig() *Config {
	return &Config{
		testMode:            false,
		testModeThreads:     3,
		testModeMinInterval: 1,
		testModeMaxInterval: 10,

		master:            false,
		configDir:         ".",
		rpcServerPort:     6421,
		rpcServerRemAdr:   "127.0.0.1:6421",
		cxoPort:           8998,
		cxoUseMemory:      false,
		cxoDir:            "bbs",
		webGUIEnable:      true,
		webGUIPort:        6420,
		webGUIOpenBrowser: true,
	}
}

// Parse fills the Config with commandline argument values.
func (c *Config) Parse() *Config {
	/*
		<<< TEST FLAGS >>>
	*/

	flag.BoolVar(&c.testMode,
		"test-mode", c.testMode,
		"whether to enable test mode")

	flag.IntVar(&c.testModeThreads,
		"test-mode-threads", c.testModeThreads,
		"number of threads to use for test mode")

	flag.IntVar(&c.testModeMinInterval,
		"test-mode-min", c.testModeMinInterval,
		"minimum interval in seconds between simulated activity")

	flag.IntVar(&c.testModeMaxInterval,
		"test-mode-max", c.testModeMaxInterval,
		"maximum interval in seconds between simulated activity")

	/*
		<<< BBS FLAGS >>>
	*/

	flag.BoolVar(&c.master,
		"master", c.master,
		"whether to enable bbs node to host boards")

	flag.StringVar(&c.configDir,
		"config-dir", c.configDir,
		"configuration directory")

	flag.IntVar(&c.rpcServerPort,
		"rpc-server-port", c.rpcServerPort,
		"port of rpc server for master node")

	flag.StringVar(&c.rpcServerRemAdr,
		"rpc-server-remote-address", c.rpcServerRemAdr,
		"remote address of rpc server for master node")

	flag.IntVar(&c.cxoPort,
		"cxo-port", c.cxoPort,
		"port of cxo daemon to connect to")

	flag.BoolVar(&c.cxoUseMemory,
		"cxo-use-memory", c.cxoUseMemory,
		"whether to use in-memory database")

	flag.StringVar(&c.cxoDir,
		"cxo-dir", c.cxoDir,
		"folder to store cxo db files in")

	flag.BoolVar(&c.webGUIEnable,
		"web-gui-enable", c.webGUIEnable,
		"whether to enable the web gui")

	flag.IntVar(&c.webGUIPort,
		"web-gui-port", c.webGUIPort,
		"local port to serve web gui on")

	flag.BoolVar(&c.webGUIOpenBrowser,
		"web-gui-open-browser", c.webGUIOpenBrowser,
		"whether to open browser after web gui is ready")

	flag.Parse()
	return c
}

// PostProcess checks the validity and post processes the flags.
func (c *Config) PostProcess() (*Config, error) {
	// Action on test mode.
	if c.testMode {
		// Check test mode settings.
		if c.testModeThreads < 0 {
			return nil, errors.New("invalid number of test mode threads specified")
		}
		if c.testModeMinInterval < 1 {
			return nil, errors.New("invalid test mode minimum interval specified")
		}
		if c.testModeMaxInterval < 1 {
			return nil, errors.New("invalid test mode maximum interval specified")
		}
		if c.testModeMinInterval > c.testModeMaxInterval {
			return nil, errors.New("test mode minimum interval > maximum interval")
		}
		// Enforce behaviour.
		c.cxoUseMemory = true
		c.configDir = "/tmp"
	}
	return c, nil
}

/*
	These functions ensure that configuration values are not accidentally modified.
*/

func (c *Config) Master() bool            { return c.master }
func (c *Config) ConfigDir() string       { return c.configDir }
func (c *Config) RPCServerPort() int      { return c.rpcServerPort }
func (c *Config) RPCServerRemAdr() string { return c.rpcServerRemAdr }
func (c *Config) CXOPort() int            { return c.cxoPort }
func (c *Config) CXOUseMemory() bool      { return c.cxoUseMemory }
func (c *Config) CXODir() string          { return c.cxoDir }
func (c *Config) WebGUIEnable() bool      { return c.webGUIEnable }
func (c *Config) WebGUIPort() int         { return c.webGUIPort }
func (c *Config) WebGUIOpenBrowser() bool { return c.webGUIOpenBrowser }
