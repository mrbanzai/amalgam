# amalgam : Opinionated wrapper for [viper](https://github.com/spf13/viper) + [pflag](https://github.com/spf13/pflag)

## Overview

Amalgam is designed to handle a particular [viper](https://github.com/spf13/viper) use-case:  loading a config file into a struct, with potential overrides via environment variables and flags.


## Basic Example

```
type MyConfig struct {
	RequestLogFile string
	ListenAddr string
	API struct {
		Endpoint string
		Timeout uint
	}
    AllowedUsers []string
    Keys map[string]string
}

config := new(MyConfig)

a, err := amalgam.New(config,
    amalgam.WithDefaultConfigFile("app.toml"),
)
if err != nil {
    panic(err)
}

// Load the config file, parse flags, etc.
if err := a.LoadFile(); err != nil {
	panic(err)
}

// `config` is populated from app.toml / environment / flags
```

In addition to loading a compatible structured config file (in any of the formats supported by viper),
this will accept the following flags:
* `--request-log-file`
* `--listen-addr`
* `--api-endpoint`
* `--api-timeout`
* `--allowed-users` (the flag can be specified multiple times, to append to the slice)

It will also accept values via these environment variables:
* `REQUESTLOGFILE`
* `LISTENADDR`
* `API_ENDPOINT`
* `API_TIMEOUT`
* `ALLOWEDUSERS` (the values should be comma-separated)

## Advanced Configurations

### Custom Flags

Amalgam makes use of the [pflag](https://github.com/spf13/pflag) library, and you have access to the flag set via
the `FlagSet` property:
```
var (
    productionMode bool
    inputFile      string
)

flags := new pflag.NewFlagSet()
flags.BoolVarP(&productionMode, "production", "p", false, "enable production mode")
flags.StringVar(&inputFile, "input", "file.xml", "data input file")

a, err := amalgam.New(...,
    amalgam.WithFlagSet(flags),
)
if err != nil {
    panic(err)
}
```
The command-line flags are parsed with the call to one of the Amalgam `Load*` methods, or by calling `Parse`
on the flagset.

You can also specify the flag name and/or description via the `amalgam` struct tag:
```
type MyConfig struct {
	RequestLogFile string `amalgam:"request-log,HTTP request log file"`
	ListenAddr string `amalgam:",Listen address (ip:port)"`
	API struct {
		Endpoint string `amalgam:",API endpoint (full URL)"`
		Timeout uint `amalgam:",Timeout for API requests, in seconds"`
	}
	FieldWithoutFlag bool `amalgam:"-"`
}
```
Tag values are comma-separated, with the flag name being the first value, and the description afterwards. To
prevent a field from being configurable via a flag, specify `-` as the flag name.  If no flag name is specified,
the default is used.

### Default Values

If you need to specify a default value for a config field / flag, just set that value in the object to be
populated with:
```
// Setup defaults
config := new(MyConfig)
config.RequestLogFile = "output.log"
config.ListenAddr = "127.0.0.1:5000"
config.API.Endpoint = "example.com:3000"
config.API.Timeout = 5
config.AllowedUsers = []string{"mporter", "mmcneil"}

a, err := amalgam.New(config)
if err != nil {
    panic(err)
}
if err := a.LoadFile(); err != nil {
    panic(err)
}
```
The default values are displayed in the usage message (when an invalid flag has been provided, or when `--help` is
provided as a flag), and are used if no value has been set.

### Options

Amalgam supports a few different options to control its operation:
```
options := &amalgam.Options{
    // Enable debugging output (writes to stderr)
	Debug: false,
    // Specifies the configuration file to use
	ConfigFile: "app.json",
    // This option suppresses the creation of the --config/-c flag to specify/override
    // the configuration file
	PreventConfigFlag: true,
    // The function to use to derive the flag names from the configuration keys.
    // Viper is case-insensitive, and uses `.` to separate nested fields, and the
    // default flag name function replaces `.` and case-changes with `-` (eg. `Sub.MyVar`
    // becomes `sub-my-var`)
	FlagNameFunc: nil, // should be a `func (string) string`
    // Allows you to specify a prefix for the associated environment variables, so
    // the below would fill the `Sub.MyVar` configuration option from `APP_SUB_MYVAR`.
	EnvPrefix: "app"
}
a := amalgam.New(options)
```

## Author

Michael Porter <michael@commercecraft.com>
